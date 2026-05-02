package internal

import (
	"database/sql"
	"fmt"
)

// ReadWords reads 1-2 character headwords from the source dictionary database,
// excluding words already present in knownHeadwords.
// Returns a slice of words to process.
func ReadWords(db *sql.DB, knownHeadwords map[string]struct{}, limit int) ([]string, error) {
	query := `SELECT DISTINCT headword FROM entries WHERE LENGTH(headword) IN (1,2)`

	if len(knownHeadwords) > 0 {
		// Build excluded words list for SQL query
		// For large known sets, we filter in Go instead of SQL
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query headwords: %w", err)
	}
	defer rows.Close()

	var words []string
	for rows.Next() {
		var word string
		if err := rows.Scan(&word); err != nil {
			return nil, fmt.Errorf("scan word: %w", err)
		}
		if _, ok := knownHeadwords[word]; ok {
			continue
		}
		words = append(words, word)
		if limit > 0 && len(words) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return words, nil
}
