package internal

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// EnrichmentEntry represents a dictionary entry with its meanings, ready for LLM analysis.
type EnrichmentEntry struct {
	EntryID   int
	Headword  string
	Pinyin    string
	Meanings  []string // concatenated from meanings.text, ordered by level/order_num
}

// EnrichmentResult is what the LLM returns for a single entry.
type EnrichmentResult struct {
	EntryID        int    `json:"entry_id"`
	PinyinIssues   string `json:"pinyin_issues"`
	PinyinFixed    string `json:"pinyin_fixed,omitempty"`
	MeaningIssues  string `json:"meaning_issues"`
	MeaningFixed   string `json:"meaning_fixed,omitempty"`
	MeaningQuality int    `json:"meaning_quality"` // 1-10
	NeedsReview    bool   `json:"needs_review"`
}

// EnrichmentRecord is what we store in the DB.
type EnrichmentRecord struct {
	EntryID         int
	Headword        string
	PinyinOriginal  string
	MeaningOriginal string
	PinyinFixed     string
	MeaningFixed    string
	PinyinIssues    string
	MeaningIssues   string
	MeaningQuality  int
	NeedsReview     bool
	CreatedAt       time.Time
}

// InitEnrichmentDB creates the enrichment tracking table.
func InitEnrichmentDB(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS enrichments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entry_id INTEGER NOT NULL UNIQUE,
			headword TEXT NOT NULL,
			pinyin_original TEXT,
			meaning_original TEXT,
			pinyin_fixed TEXT,
			meaning_fixed TEXT,
			pinyin_issues TEXT DEFAULT '',
			meaning_issues TEXT DEFAULT '',
			meaning_quality INTEGER DEFAULT 0,
			needs_review INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_enrichments_entry ON enrichments(entry_id)`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("exec %q: %w", q[:40], err)
		}
	}
	return nil
}

// ReadEnrichedEntryIDs returns a set of entry IDs already processed.
func ReadEnrichedEntryIDs(db *sql.DB) (map[int]struct{}, error) {
	rows, err := db.Query(`SELECT entry_id FROM enrichments`)
	if err != nil {
		return nil, fmt.Errorf("query enriched entries: %w", err)
	}
	defer rows.Close()

	known := make(map[int]struct{})
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		known[id] = struct{}{}
	}
	return known, rows.Err()
}

// ReadEntriesForEnrichment reads entries with their meanings, skipping already-processed ones.
// limit: 0 = all.
func ReadEntriesForEnrichment(sourceDB *sql.DB, known map[int]struct{}, limit int) ([]EnrichmentEntry, error) {
	rows, err := sourceDB.Query(`
		SELECT e.id, e.headword, e.pinyin
		FROM entries e
		ORDER BY e.id
	`)
	if err != nil {
		return nil, fmt.Errorf("query entries: %w", err)
	}
	defer rows.Close()

	type entry struct {
		id      int
		headword string
		pinyin   string
	}
	var bare []entry

	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.headword, &e.pinyin); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if _, ok := known[e.id]; ok {
			continue
		}
		bare = append(bare, e)
		if limit > 0 && len(bare) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Batch-fetch meanings for all retrieved entries
	entries := make([]EnrichmentEntry, 0, len(bare))
	for _, b := range bare {
		e := EnrichmentEntry{
			EntryID:  b.id,
			Headword: b.headword,
			Pinyin:   b.pinyin,
		}

		mRows, err := sourceDB.Query(`
			SELECT text FROM meanings
			WHERE entry_id = ?
			ORDER BY level, order_num
		`, b.id)
		if err != nil {
			return nil, fmt.Errorf("query meanings for entry %d: %w", b.id, err)
		}

		for mRows.Next() {
			var text string
			if err := mRows.Scan(&text); err != nil {
				mRows.Close()
				return nil, fmt.Errorf("scan meaning: %w", err)
			}
			e.Meanings = append(e.Meanings, text)
		}
		mRows.Close()

		entries = append(entries, e)
	}

	return entries, nil
}

// InsertEnrichmentResults batch-inserts LLM enrichment results.
func InsertEnrichmentResults(db *sql.DB, results []EnrichmentResult, entries []EnrichmentEntry) error {
	if len(results) == 0 {
		return nil
	}

	// Build a lookup for original data
	orig := make(map[int]*EnrichmentEntry, len(entries))
	for i := range entries {
		orig[entries[i].EntryID] = &entries[i]
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO enrichments
			(entry_id, headword, pinyin_original, meaning_original,
			 pinyin_fixed, meaning_fixed, pinyin_issues, meaning_issues,
			 meaning_quality, needs_review)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(entry_id) DO UPDATE SET
			pinyin_fixed = excluded.pinyin_fixed,
			meaning_fixed = excluded.meaning_fixed,
			pinyin_issues = excluded.pinyin_issues,
			meaning_issues = excluded.meaning_issues,
			meaning_quality = excluded.meaning_quality,
			needs_review = excluded.needs_review
	`)
	if err != nil {
		return fmt.Errorf("prepare stmt: %w", err)
	}
	defer stmt.Close()

	for _, r := range results {
		o := orig[r.EntryID]
		if o == nil {
			continue
		}
		origMeanings := strings.Join(o.Meanings, " | ")
		rv := 0
		if r.NeedsReview {
			rv = 1
		}
		if _, err := stmt.Exec(
			r.EntryID, o.Headword, o.Pinyin, origMeanings,
			r.PinyinFixed, r.MeaningFixed,
			r.PinyinIssues, r.MeaningIssues,
			r.MeaningQuality, rv,
		); err != nil {
			return fmt.Errorf("insert entry %d: %w", r.EntryID, err)
		}
	}

	return tx.Commit()
}
