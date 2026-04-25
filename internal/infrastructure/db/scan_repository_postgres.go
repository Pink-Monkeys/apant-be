package db

import (
	"context"
	"fmt"

	"apant_be/internal/domain"
)

type ScanRepositoryPostgres struct {
	db *Postgres
}

func NewScanRepositoryPostgres(db *Postgres) *ScanRepositoryPostgres {
	return &ScanRepositoryPostgres{db: db}
}

func (r *ScanRepositoryPostgres) Save(_ context.Context, _ domain.Scan) error {
	return fmt.Errorf("not implemented")
}

func (r *ScanRepositoryPostgres) FindByID(_ context.Context, _ string) (domain.Scan, error) {
	return domain.Scan{}, fmt.Errorf("not implemented")
}
