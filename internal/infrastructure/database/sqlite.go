package database

import (
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// CreateNewDB создаёт новую БД с полной схемой (entries + meanings + examples + enrich_log).
func CreateNewDB(dbPath string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	schema := `
	
	CREATE TABLE IF NOT EXISTS examples (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		headword TEXT NOT NULL,
		text TEXT NOT NULL
	);

	`

	db.MustExec(schema)

	return db, nil
}

// OpenSourceDB открывает существующую SQLite-БД только для чтения.
func OpenSourceDB(dbPath string) (*sqlx.DB, error) {
	log.Printf("Opening source DB (read-only): %s", dbPath)
	db, err := sqlx.Connect("sqlite3", dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}
