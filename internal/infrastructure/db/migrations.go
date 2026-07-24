package db

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

const schemaMigrationsTable = "gorm_migrations"

// SeedConfig carries values that data-seed migrations need from the environment.
// It is empty for rollbacks and schema-only runs; seed migrations no-op when the
// relevant fields are absent.
type SeedConfig struct {
	// EncryptKey encrypts a plaintext secret to ciphertext for storage. Nil
	// disables the LLM provider seed (a missing/invalid LLM_ENCRYPTION_KEY).
	EncryptKey func(plaintext string) ([]byte, error)

	OpenAIAPIKey   string
	OpenAIModel    string
	AnthropicKey   string
	AnthropicModel string

	// Admin bootstrap. AdminPasswordHash is a pre-computed bcrypt hash so this
	// package stays free of auth dependencies.
	AdminUsername     string
	AdminEmail        string
	AdminPasswordHash string
}

func ApplyMigrations(gormDB *gorm.DB, seed SeedConfig) error {
	if gormDB == nil {
		return fmt.Errorf("gorm db is nil")
	}

	m := gormigrate.New(gormDB, &gormigrate.Options{
		TableName: schemaMigrationsTable,
	}, migrations(seed))

	return m.Migrate()
}

func RollbackLastMigration(gormDB *gorm.DB) error {
	if gormDB == nil {
		return fmt.Errorf("gorm db is nil")
	}

	m := gormigrate.New(gormDB, &gormigrate.Options{
		TableName: schemaMigrationsTable,
	}, migrations(SeedConfig{}))

	return m.RollbackLast()
}

func RollbackToMigration(gormDB *gorm.DB, migrationID string) error {
	if gormDB == nil {
		return fmt.Errorf("gorm db is nil")
	}

	m := gormigrate.New(gormDB, &gormigrate.Options{
		TableName: schemaMigrationsTable,
	}, migrations(SeedConfig{}))

	return m.RollbackTo(migrationID)
}

func migrations(seed SeedConfig) []*gormigrate.Migration {
	return []*gormigrate.Migration{
		{
			ID: "20260429_create_auth_tables",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&userModel{}, &refreshTokenModel{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&refreshTokenModel{}, &userModel{})
			},
		},
		{
			ID: "20260429_add_email_to_users",
			Migrate: func(tx *gorm.DB) error {
				type userEmailPatch struct {
					Email string `gorm:"column:email;type:text"`
				}
				return tx.Table("users").AutoMigrate(&userEmailPatch{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn("users", "email")
			},
		},
		{
			ID: "20260513_add_unique_index_on_users_email",
			Migrate: func(tx *gorm.DB) error {
				return tx.Exec(
					"CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email" +
						" ON users (email)" +
						" WHERE email IS NOT NULL AND email <> ''",
				).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Exec("DROP INDEX IF EXISTS idx_users_email").Error
			},
		},
		{
			ID: "20260602_create_reports_table",
			Migrate: func(tx *gorm.DB) error {
				return migrateReports(tx)
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&reportModel{})
			},
		},
		{
			ID: "20260608_create_scans_table",
			Migrate: func(tx *gorm.DB) error {
				return migrateScans(tx)
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&scanModel{})
			},
		},
		{
			ID: "20260608_add_scan_id_to_reports",
			Migrate: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&reportModel{}, "scan_id") {
					return nil
				}
				return tx.Migrator().AddColumn(&reportModel{}, "ScanID")
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn(&reportModel{}, "scan_id")
			},
		},
		{
			ID: "20260609_add_target_info_to_scans",
			Migrate: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&scanModel{}, "target_info") {
					return nil
				}
				return tx.Migrator().AddColumn(&scanModel{}, "TargetInfo")
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn(&scanModel{}, "target_info")
			},
		},
		{
			ID: "20260609_add_description_to_scans",
			Migrate: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&scanModel{}, "description") {
					return nil
				}
				return tx.Migrator().AddColumn(&scanModel{}, "Description")
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn(&scanModel{}, "description")
			},
		},
		{
			ID: "20260610_add_scan_type_to_scans",
			Migrate: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&scanModel{}, "scan_type") {
					return nil
				}
				return tx.Migrator().AddColumn(&scanModel{}, "ScanType")
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn(&scanModel{}, "scan_type")
			},
		},
		{
			ID: "20260610_add_mitigation_to_reports",
			Migrate: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&reportModel{}, "mitigation") {
					return nil
				}
				return tx.Migrator().AddColumn(&reportModel{}, "Mitigation")
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn(&reportModel{}, "mitigation")
			},
		},
		{
			ID: "20260622_add_session_started_at_to_refresh_tokens",
			Migrate: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&refreshTokenModel{}, "session_started_at") {
					return nil
				}
				return tx.Migrator().AddColumn(&refreshTokenModel{}, "SessionStartedAt")
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn(&refreshTokenModel{}, "session_started_at")
			},
		},
		{
			ID: "20260610_split_reports_data_columns",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&reportModel{}); err != nil {
					return err
				}
				if !tx.Migrator().HasColumn(&reportModel{}, "data") {
					return nil
				}
				if err := tx.Exec(`UPDATE reports SET
					title = data->>'title',
					overall_severity = data->>'overall_severity',
					executive_summary = data->>'executive_summary',
					conclusion = data->>'conclusion',
					metadata = data->'metadata',
					target_info = data->'target_info',
					attack_surface = data->'attack_surface',
					vulnerabilities = data->'vulnerabilities',
					statistics = data->'statistics'`).Error; err != nil {
					return err
				}
				return tx.Exec(`ALTER TABLE reports DROP COLUMN IF EXISTS data`).Error
			},
			Rollback: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&reportModel{}, "data") {
					return nil
				}
				if err := tx.Exec(`ALTER TABLE reports ADD COLUMN data jsonb`).Error; err != nil {
					return err
				}
				if err := tx.Exec(`UPDATE reports SET data = jsonb_build_object(
					'title', title,
					'overall_severity', overall_severity,
					'metadata', metadata,
					'executive_summary', executive_summary,
					'target_info', target_info,
					'attack_surface', attack_surface,
					'vulnerabilities', vulnerabilities,
					'statistics', statistics,
					'conclusion', conclusion
				)`).Error; err != nil {
					return err
				}
				for _, col := range []string{
					"title", "overall_severity", "executive_summary", "conclusion",
					"metadata", "target_info", "attack_surface", "vulnerabilities", "statistics",
				} {
					if err := tx.Exec("ALTER TABLE reports DROP COLUMN IF EXISTS " + col).Error; err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			ID: "20260704_add_error_to_scans",
			Migrate: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&scanModel{}, "error") {
					return nil
				}
				return tx.Migrator().AddColumn(&scanModel{}, "Error")
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn(&scanModel{}, "error")
			},
		},
		{
			ID: "20260704_create_llm_tables",
			Migrate: func(tx *gorm.DB) error {
				return migrateLLM(tx)
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&llmModelModel{}, &llmProviderModel{})
			},
		},
		{
			ID: "20260704_seed_llm_from_env",
			Migrate: func(tx *gorm.DB) error {
				return seedLLMFromEnv(tx, seed)
			},
			// Data seed: nothing to roll back structurally. Leaving seeded rows in
			// place is safe and avoids clobbering admin-edited providers.
			Rollback: func(tx *gorm.DB) error { return nil },
		},
		{
			ID: "20260704_seed_admin_user",
			Migrate: func(tx *gorm.DB) error {
				return seedAdminUser(tx, seed)
			},
			Rollback: func(tx *gorm.DB) error { return nil },
		},
		{
			ID: "20260705_add_username_to_reports",
			Migrate: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&reportModel{}, "username") {
					return nil
				}
				return tx.Migrator().AddColumn(&reportModel{}, "Username")
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn(&reportModel{}, "username")
			},
		},
		{
			ID: "20260705_add_username_to_scans",
			Migrate: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&scanModel{}, "username") {
					return nil
				}
				return tx.Migrator().AddColumn(&scanModel{}, "Username")
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn(&scanModel{}, "username")
			},
		},
		{
			ID: "20260714_create_user_llm_preferences",
			Migrate: func(tx *gorm.DB) error {
				if err := migrateUserLLMPreferences(tx); err != nil {
					return err
				}
				// Cascade delete the preference when its user is removed. Added as a
				// named FK so it can be dropped on rollback; guarded by existence so a
				// re-run is a no-op.
				if !tx.Migrator().HasConstraint(&userLLMPreferenceModel{}, "fk_user_llm_preferences_user") {
					return tx.Exec(`ALTER TABLE user_llm_preferences
						ADD CONSTRAINT fk_user_llm_preferences_user
						FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE`).Error
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&userLLMPreferenceModel{})
			},
		},
		{
			ID: "20260723_add_price_to_llm_models",
			Migrate: func(tx *gorm.DB) error {
				cols := []struct{ field, column string }{
					{"PriceInPer1M", "price_in_per_1m"},
					{"PriceOutPer1M", "price_out_per_1m"},
					{"Currency", "currency"},
				}
				for _, c := range cols {
					if tx.Migrator().HasColumn(&llmModelModel{}, c.column) {
						continue
					}
					if err := tx.Migrator().AddColumn(&llmModelModel{}, c.field); err != nil {
						return err
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				for _, col := range []string{"price_in_per_1m", "price_out_per_1m", "currency"} {
					if err := tx.Migrator().DropColumn(&llmModelModel{}, col); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			ID: "20260723_add_token_usage_and_price_to_scans",
			Migrate: func(tx *gorm.DB) error {
				cols := []struct{ field, column string }{
					{"InputTokens", "input_tokens"},
					{"OutputTokens", "output_tokens"},
					{"TotalTokens", "total_tokens"},
					{"Calls", "calls"},
					{"PriceInPer1M", "price_in_per_1m"},
					{"PriceOutPer1M", "price_out_per_1m"},
					{"PriceCurrency", "price_currency"},
				}
				for _, c := range cols {
					if tx.Migrator().HasColumn(&scanModel{}, c.column) {
						continue
					}
					if err := tx.Migrator().AddColumn(&scanModel{}, c.field); err != nil {
						return err
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				for _, col := range []string{
					"input_tokens", "output_tokens", "total_tokens", "calls",
					"price_in_per_1m", "price_out_per_1m", "price_currency",
				} {
					if err := tx.Migrator().DropColumn(&scanModel{}, col); err != nil {
						return err
					}
				}
				return nil
			},
		},
	}
}
