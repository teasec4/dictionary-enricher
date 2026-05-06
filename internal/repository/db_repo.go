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


// For CLI
func (r *DBRepo) GetExamples(limit int) ([]domain.Example, error) {
        var examples []domain.Example
        err := r.db.Select(&examples, `SELECT headword, text FROM examples LIMIT ?`, limit)
        return examples, err
}

func (r *DBRepo) BulkInsert(ctx context.Context, examples []domain.Example) error {
        if len(examples) == 0 {
                return nil
        }

        tx, err := r.db.BeginTx(ctx, nil)
        if err != nil {
                return fmt.Errorf("begin tx: %w", err)
        }
        defer tx.Rollback()

        stmt, err := tx.PrepareContext(ctx, `INSERT INTO examples (headword, text) VALUES (?, ?)`)
        if err != nil {
                return fmt.Errorf("prepare: %w", err)
        }
        defer stmt.Close()

        for _, ex := range examples {
                _, err := stmt.ExecContext(ctx, ex.Headword, ex.Text)
                if err != nil {
                        return fmt.Errorf("insert: %w", err)
                }
        }

        if err := tx.Commit(); err != nil {
                return fmt.Errorf("commit: %w", err)
        }

        return nil
}


