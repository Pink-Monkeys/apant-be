package user

import (
	"context"
	"testing"
	"time"

	"apant_be/internal/domain"
	"apant_be/internal/infrastructure/db"
)

func seedUser(t *testing.T, repo domain.UserRepository, id, username, role string) {
	t.Helper()
	err := repo.Create(context.Background(), domain.User{
		ID:           id,
		Username:     username,
		Email:        username + "@test.local",
		PasswordHash: "x",
		Role:         role,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})
	if err != nil {
		t.Fatalf("seed %s: %v", username, err)
	}
}

func TestCreate(t *testing.T) {
	ctx := context.Background()

	newSvc := func() (*Service, domain.UserRepository) {
		repo := db.NewMemoryUserRepository()
		return NewService(repo), repo
	}

	t.Run("creates pentester by default", func(t *testing.T) {
		svc, repo := newSvc()
		got, err := svc.Create(ctx, CreateUserRequest{
			Username: "andi", Email: "andi@test.local", Password: "AndiPass123",
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if got.Role != domain.RolePentester {
			t.Fatalf("role = %s, want pentester", got.Role)
		}
		if _, found, _ := repo.FindByUsername(ctx, "andi"); !found {
			t.Fatal("user should be persisted")
		}
	})

	t.Run("creates admin when requested", func(t *testing.T) {
		svc, _ := newSvc()
		got, err := svc.Create(ctx, CreateUserRequest{
			Username: "bossadmin", Email: "boss@test.local", Password: "BossPass123", Role: "admin",
		})
		if err != nil {
			t.Fatalf("create admin: %v", err)
		}
		if got.Role != domain.RoleAdmin {
			t.Fatalf("role = %s, want admin", got.Role)
		}
	})

	t.Run("duplicate username rejected", func(t *testing.T) {
		svc, _ := newSvc()
		req := CreateUserRequest{Username: "dupe", Email: "dupe1@test.local", Password: "DupePass123"}
		if _, err := svc.Create(ctx, req); err != nil {
			t.Fatalf("first create: %v", err)
		}
		req.Email = "dupe2@test.local"
		if _, err := svc.Create(ctx, req); err == nil {
			t.Fatal("expected duplicate username rejected")
		}
	})

	t.Run("weak password rejected", func(t *testing.T) {
		svc, _ := newSvc()
		if _, err := svc.Create(ctx, CreateUserRequest{
			Username: "weak", Email: "weak@test.local", Password: "short",
		}); err == nil {
			t.Fatal("expected weak password rejected")
		}
	})

	t.Run("invalid role rejected", func(t *testing.T) {
		svc, _ := newSvc()
		if _, err := svc.Create(ctx, CreateUserRequest{
			Username: "roley", Email: "roley@test.local", Password: "RoleyPass123", Role: "superuser",
		}); err == nil {
			t.Fatal("expected invalid role rejected")
		}
	})
}

func TestUpdateRole(t *testing.T) {
	ctx := context.Background()

	t.Run("cannot change own role", func(t *testing.T) {
		repo := db.NewMemoryUserRepository()
		seedUser(t, repo, "admin1", "admin1", domain.RoleAdmin)
		svc := NewService(repo)
		if _, err := svc.UpdateRole(ctx, "admin1", "admin1", domain.RolePentester); err == nil {
			t.Fatal("expected error changing own role")
		}
	})

	t.Run("invalid role rejected", func(t *testing.T) {
		repo := db.NewMemoryUserRepository()
		seedUser(t, repo, "admin1", "admin1", domain.RoleAdmin)
		seedUser(t, repo, "p1", "p1", domain.RolePentester)
		svc := NewService(repo)
		if _, err := svc.UpdateRole(ctx, "admin1", "p1", "superuser"); err == nil {
			t.Fatal("expected invalid role rejected")
		}
	})

	t.Run("promote pentester to admin, revokes their sessions", func(t *testing.T) {
		repo := db.NewMemoryUserRepository()
		seedUser(t, repo, "admin1", "admin1", domain.RoleAdmin)
		seedUser(t, repo, "p1", "p1", domain.RolePentester)
		_ = repo.CreateRefreshToken(ctx, domain.RefreshToken{
			ID: "tok1", UserID: "p1", TokenHash: "h1",
			ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now(),
		})
		svc := NewService(repo)

		got, err := svc.UpdateRole(ctx, "admin1", "p1", domain.RoleAdmin)
		if err != nil {
			t.Fatalf("promote: %v", err)
		}
		if got.Role != domain.RoleAdmin {
			t.Fatalf("role = %s, want admin", got.Role)
		}
		// Session must be revoked so the old (pentester) token cannot linger.
		tok, _, _ := repo.FindRefreshTokenByHash(ctx, "h1")
		if tok.RevokedAt == nil {
			t.Fatal("expected the promoted user's refresh token to be revoked")
		}
	})

	t.Run("demote allowed while another admin remains", func(t *testing.T) {
		repo := db.NewMemoryUserRepository()
		seedUser(t, repo, "admin1", "admin1", domain.RoleAdmin)
		seedUser(t, repo, "admin2", "admin2", domain.RoleAdmin)
		svc := NewService(repo)
		if _, err := svc.UpdateRole(ctx, "admin1", "admin2", domain.RolePentester); err != nil {
			t.Fatalf("demote with 2 admins should pass: %v", err)
		}
	})
}

// TestEnsureNotLastAdmin exercises the last-admin guard directly, since reaching
// it through UpdateRole/Delete requires a second admin as the actor.
func TestEnsureNotLastAdmin(t *testing.T) {
	ctx := context.Background()

	repo := db.NewMemoryUserRepository()
	seedUser(t, repo, "admin1", "admin1", domain.RoleAdmin)
	svc := NewService(repo)
	if err := svc.ensureNotLastAdmin(ctx); err == nil {
		t.Fatal("with 1 admin, guard must block")
	}

	seedUser(t, repo, "admin2", "admin2", domain.RoleAdmin)
	if err := svc.ensureNotLastAdmin(ctx); err != nil {
		t.Fatalf("with 2 admins, guard must allow: %v", err)
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("cannot delete self", func(t *testing.T) {
		repo := db.NewMemoryUserRepository()
		seedUser(t, repo, "admin1", "admin1", domain.RoleAdmin)
		svc := NewService(repo)
		if err := svc.Delete(ctx, "admin1", "admin1"); err == nil {
			t.Fatal("expected self-delete blocked")
		}
	})

	t.Run("delete pentester works", func(t *testing.T) {
		repo := db.NewMemoryUserRepository()
		seedUser(t, repo, "admin1", "admin1", domain.RoleAdmin)
		seedUser(t, repo, "p1", "p1", domain.RolePentester)
		svc := NewService(repo)
		if err := svc.Delete(ctx, "admin1", "p1"); err != nil {
			t.Fatalf("delete pentester: %v", err)
		}
		if _, found, _ := repo.FindByID(ctx, "p1"); found {
			t.Fatal("pentester should be gone")
		}
	})

	t.Run("deleting an admin is blocked when it is the last one", func(t *testing.T) {
		repo := db.NewMemoryUserRepository()
		seedUser(t, repo, "admin1", "admin1", domain.RoleAdmin)
		seedUser(t, repo, "admin2", "admin2", domain.RoleAdmin)
		svc := NewService(repo)
		// admin1 deletes admin2 → allowed (leaves admin1).
		if err := svc.Delete(ctx, "admin1", "admin2"); err != nil {
			t.Fatalf("deleting with 2 admins should pass: %v", err)
		}
		// Re-add a second admin so admin1 can attempt to delete it, leaving one.
		seedUser(t, repo, "admin3", "admin3", domain.RoleAdmin)
		if err := svc.Delete(ctx, "admin1", "admin3"); err != nil {
			t.Fatalf("deleting down to 1 admin should pass: %v", err)
		}
		// Now admin1 is the only admin; deleting it is blocked (self-guard here, but
		// the last-admin count guard is verified in TestEnsureNotLastAdmin).
		if err := svc.Delete(ctx, "admin1", "admin1"); err == nil {
			t.Fatal("expected deleting the sole admin to be blocked")
		}
	})
}

func TestResetPassword(t *testing.T) {
	ctx := context.Background()
	repo := db.NewMemoryUserRepository()
	seedUser(t, repo, "p1", "p1", domain.RolePentester)
	_ = repo.CreateRefreshToken(ctx, domain.RefreshToken{
		ID: "tok1", UserID: "p1", TokenHash: "h1",
		ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now(),
	})
	svc := NewService(repo)

	if err := svc.ResetPassword(ctx, "p1", "short"); err == nil {
		t.Fatal("expected weak password rejected")
	}
	if err := svc.ResetPassword(ctx, "missing", "ValidPass123"); err == nil {
		t.Fatal("expected not-found for missing user")
	}
	if err := svc.ResetPassword(ctx, "p1", "ValidPass123"); err != nil {
		t.Fatalf("valid reset: %v", err)
	}
	// Reset must revoke sessions.
	tok, _, _ := repo.FindRefreshTokenByHash(ctx, "h1")
	if tok.RevokedAt == nil {
		t.Fatal("expected reset to revoke the user's refresh token")
	}
}
