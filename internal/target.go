package internal

import (
	"database/sql"
	"fmt"
)

// InitTargetDB creates the examples table and index if they don't exist.
func InitTargetDB(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS examples (
			headword TEXT NOT NULL,
			example TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_examples_headword ON examples(headword)`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("exec %q: %w", q[:40], err)
		}
	}
	return nil
}

// ReadKnownHeadwords returns a set of headwords that already have examples.
func ReadKnownHeadwords(db *sql.DB) (map[string]struct{}, error) {
	rows, err := db.Query(`SELECT DISTINCT headword FROM examples`)
	if err != nil {
		return nil, fmt.Errorf("query known headwords: %w", err)
	}
	defer rows.Close()

	known := make(map[string]struct{})
	for rows.Next() {
		var word string
		if err := rows.Scan(&word); err != nil {
			return nil, fmt.Errorf("scan known headword: %w", err)
		}
		known[word] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate known headwords: %w", err)
	}

	return known, nil
}

// InsertExamples batch-inserts example rows.
func InsertExamples(db *sql.DB, rows []ExampleResult) error {
	if len(rows) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO examples(headword, example) VALUES(?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare stmt: %w", err)
	}
	defer stmt.Close()

	for _, r := range rows {
		for _, ex := range r.Examples {
			if _, err := stmt.Exec(r.Headword, ex); err != nil {
				return fmt.Errorf("insert %q: %w", r.Headword, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

// ExampleResult holds a headword and its generated examples.
type ExampleResult struct {
	Headword string
	Examples []string
}
