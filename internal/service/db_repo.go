package service

import (
	"context"
	"dabkrs-examples/internal/domain"
	"time"

	"github.com/jmoiron/sqlx"
)

type DBRepo struct {
	db *sqlx.DB
}

func NewRepo(db *sqlx.DB) *DBRepo {
	return &DBRepo{
		db: db,
	}
}

func (r *DBRepo) Create(ctx context.Context, e *domain.Entry) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	// rollback если что-то пошло не так
	defer tx.Rollback()

	// 1. insert entry
	res, err := tx.ExecContext(ctx,
		`INSERT INTO entries (headword, pinyin) VALUES (?, ?)`,
		e.Hanzi, e.Pinyin,
	)
	if err != nil {
		return err
	}

	entryID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	e.ID = int(entryID)

	// 2. insert meanings
	for i := range e.Meanings {
		m := &e.Meanings[i]

		res, err := tx.ExecContext(ctx,
			`INSERT INTO meanings (entry_id, text) VALUES (?, ?)`,
			entryID, m.Text,
		)
		if err != nil {
			return err
		}

		meaningID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		m.ID = int(meaningID)

	}

	// commit
	return tx.Commit()
}

func (r *DBRepo) List(ctx context.Context, limit, offset int) ([]domain.Entry, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var entires []domain.Entry
	err := r.db.SelectContext(ctx, &entires,
		`SELECT id, headword, pinyin FROM entries ORDER BY id LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}

	for i := range entires {
		var meanings []domain.Meaning
		err := r.db.SelectContext(ctx, &meanings,
			`SELECT id, text FROM meanings WHERE entry_id = ? ORDER BY id`,
			entires[i].ID,
		)
		if err != nil {
			return nil, err
		}
		entires[i].Meanings = meanings
	}

	return entires, nil
}

func (r *DBRepo) Count(ctx context.Context) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var count int
	err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM entries`)
	if err != nil {
		return 0, err
	}

	return count, nil
}
