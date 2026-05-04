package repository

import (
	"context"
	"dabkrs-examples/internal/domain"
	"time"

	"github.com/jmoiron/sqlx"
)

type ExampleRepo struct{
	db *sqlx.DB
}

func NewExampleRepo(db *sqlx.DB) *ExampleRepo{
	return &ExampleRepo{
		db: db,
	}
}

func (r *ExampleRepo) Create(ctx context.Context, e *domain.Example) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO examples (headword, text) VALUES (?, ?)`,
		e.Headword, e.Text,
	)
	if err != nil {
		return err
	}

	
	return nil
}


