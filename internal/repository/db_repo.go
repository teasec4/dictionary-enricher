package repository

import (
	"context"
	"dabkrs-examples/internal/domain"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type DBRepo struct{
	db *sqlx.DB
}

func NewDBRepo(db *sqlx.DB) *DBRepo{
	return &DBRepo{
		db: db,
	}
}

func (r *DBRepo) Create(ctx context.Context, e *domain.Example) error {
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

type HeadwordWithID struct {
    ID       int    `db:"id"`
    Headword string `db:"headword"`
}

func (r *DBRepo) LoadHeadwords(ctx context.Context, limit int, afterID int) ([]HeadwordWithID, error) {
        var headwords []HeadwordWithID
        query := `SELECT id, headword FROM entries WHERE id > ? ORDER BY id LIMIT ?`
        err := r.db.SelectContext(ctx, &headwords, query, afterID, limit)
        if err != nil {
            return nil, fmt.Errorf("load headwords: %w", err)
        }
        return headwords, nil
    }

func (r *DBRepo) GetMaxID(ctx context.Context) (int, error) {
        var id int
        err := r.db.GetContext(ctx, &id, `SELECT MAX(id) FROM entries`)
        return id, err
}


