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

func (r *DBRepo) GetEntries(limit int) ([]domain.Entry, error) {
        var entries []domain.Entry
        err := r.db.Select(&entries, `SELECT id, headword, pinyin FROM entries LIMIT ?`, limit)
        return entries, err
}

func (r *DBRepo) GetByHeadword(headword string) (*domain.Entry, error) {
        var entry domain.Entry
        err := r.db.Get(&entry, `SELECT id, headword, pinyin, pinyin_normalized FROM entries WHERE headword = ?`, headword)
        if err != nil {
                return nil, err
        }

        var meanings []domain.Meaning
        r.db.Select(&meanings, `SELECT id, text FROM meanings WHERE entry_id = ?`, entry.ID)
        entry.Meanings = meanings

        return &entry, nil
}

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


