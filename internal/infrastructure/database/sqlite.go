package database

import (
	"log"
	"strings"
	"unicode"

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

// NormalizePinyin: убирает тоны, lowercase, без пробелов.
// "Nǐ hǎo" → "ni hao" → "nihao"
func NormalizePinyin(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		switch {
		case r == ' ':
			continue
		case r >= 'A' && r <= 'Z':
			b.WriteRune(unicode.ToLower(r))
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r == 'ā' || r == 'á' || r == 'ǎ' || r == 'à':
			b.WriteByte('a')
		case r == 'ē' || r == 'é' || r == 'ě' || r == 'è':
			b.WriteByte('e')
		case r == 'ī' || r == 'í' || r == 'ǐ' || r == 'ì':
			b.WriteByte('i')
		case r == 'ō' || r == 'ó' || r == 'ǒ' || r == 'ò':
			b.WriteByte('o')
		case r == 'ū' || r == 'ú' || r == 'ǔ' || r == 'ù':
			b.WriteByte('u')
		case r == 'ǖ' || r == 'ǘ' || r == 'ǚ' || r == 'ǜ':
			b.WriteByte('v')
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}
