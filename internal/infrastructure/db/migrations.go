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
	}
}
