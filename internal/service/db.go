package service

import (
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func CreateNewDB(dbPath string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS entries (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    headword TEXT NOT NULL UNIQUE,
	    pinyin TEXT,
	    pinyin_normalized TEXT
	);

	CREATE TABLE IF NOT EXISTS meanings (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    entry_id INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
	    level INTEGER DEFAULT 0,
	    text TEXT NOT NULL,
	    order_num INTEGER DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_meanings_entry ON meanings(entry_id);
	`

	db.MustExec(schema)

	return db, nil
}

func Open(driver, dsn string) (*sqlx.DB, error) {
	log.Printf("Openning source DB: %s", dsn)
	db, err := sqlx.Connect(driver, dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	return db, nil
}
