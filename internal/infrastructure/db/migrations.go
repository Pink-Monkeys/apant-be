package db

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

const schemaMigrationsTable = "gorm_migrations"

func ApplyMigrations(gormDB *gorm.DB) error {
	if gormDB == nil {
		return fmt.Errorf("gorm db is nil")
	}

	m := gormigrate.New(gormDB, &gormigrate.Options{
		TableName: schemaMigrationsTable,
	}, migrations())

	return m.Migrate()
}

func RollbackLastMigration(gormDB *gorm.DB) error {
	if gormDB == nil {
		return fmt.Errorf("gorm db is nil")
	}

	m := gormigrate.New(gormDB, &gormigrate.Options{
		TableName: schemaMigrationsTable,
	}, migrations())

	return m.RollbackLast()
}

func RollbackToMigration(gormDB *gorm.DB, migrationID string) error {
	if gormDB == nil {
		return fmt.Errorf("gorm db is nil")
	}

	m := gormigrate.New(gormDB, &gormigrate.Options{
		TableName: schemaMigrationsTable,
	}, migrations())

	return m.RollbackTo(migrationID)
}

func migrations() []*gormigrate.Migration {
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
	}
}
