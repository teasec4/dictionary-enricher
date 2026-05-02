package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// EnricherConfig holds configuration for the dictionary enrichment pipeline.
type EnricherConfig struct {
	SourcePath string // path to dictionary.db (read-only source)
	TargetPath string // path to enrichments.db (or reuse dictionary.db for enrichment table)
	LLMBaseURL string
	LLMModel   string
	Workers    int
	BatchSize  int // entries per LLM call
	Limit      int // max entries to process (0 = all)
}

// Enricher orchestrates dictionary cleaning and enrichment via LLM.
type Enricher struct {
	cfg EnricherConfig
}

// NewEnricher creates a new enricher.
func NewEnricher(cfg EnricherConfig) *Enricher {
	if cfg.Workers <= 0 {
		cfg.Workers = 5
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 20
	}
	return &Enricher{cfg: cfg}
}

// Run executes the enrichment pipeline.
func (e *Enricher) Run(ctx context.Context) error {
	// ── Open source DB (read-only) ──
	log.Printf("Opening source DB: %s", e.cfg.SourcePath)
	sourceDB, err := sql.Open("sqlite3", e.cfg.SourcePath+"?mode=ro")
	if err != nil {
		return fmt.Errorf("open source DB: %w", err)
	}
	defer sourceDB.Close()

	// ── Open target DB ──
	targetDB, err := sql.Open("sqlite3", e.cfg.TargetPath)
	if err != nil {
		return fmt.Errorf("open target DB: %w", err)
	}
	defer targetDB.Close()

	if err := InitEnrichmentDB(targetDB); err != nil {
		return fmt.Errorf("init enrichments table: %w", err)
	}

	// ── Read already processed ──
	known, err := ReadEnrichedEntryIDs(targetDB)
	if err != nil {
		return fmt.Errorf("read enriched IDs: %w", err)
	}
	log.Printf("Already enriched: %d entries", len(known))

	// ── Read entries ──
	log.Println("Reading entries with meanings...")
	entries, err := ReadEntriesForEnrichment(sourceDB, known, e.cfg.Limit)
	if err != nil {
		return fmt.Errorf("read entries: %w", err)
	}
	log.Printf("Entries to process: %d", len(entries))

	if len(entries) == 0 {
		log.Println("Nothing to process.")
		return nil
	}

	llm := NewLLMClient(LLMConfig{
		BaseURL: e.cfg.LLMBaseURL,
		Model:   e.cfg.LLMModel,
	})

	// ── Split into batches ──
	batches := make([][]EnrichmentEntry, 0, (len(entries)+e.cfg.BatchSize-1)/e.cfg.BatchSize)
	for i := 0; i < len(entries); i += e.cfg.BatchSize {
		end := i + e.cfg.BatchSize
		if end > len(entries) {
			end = len(entries)
		}
		batches = append(batches, entries[i:end])
	}
	log.Printf("Total batches: %d (batch size: %d)", len(batches), e.cfg.BatchSize)

	// ── Progress tracking ──
	var processed, failed atomic.Int64
	startTime := time.Now()
	total := len(batches)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go reportProgress(ctx, "Enrich", &processed, &failed, total, startTime)

	// ── Worker pool ──
	type batchJob struct {
		index   int
		entries []EnrichmentEntry
	}
	type batchResult struct {
		index   int
		results []EnrichmentResult
		err     error
	}

	jobChan := make(chan batchJob, len(batches))
	resultChan := make(chan batchResult, len(batches))

	go func() {
		for i, batch := range batches {
			jobChan <- batchJob{index: i, entries: batch}
		}
		close(jobChan)
	}()

	var wg sync.WaitGroup
	for w := 0; w < e.cfg.Workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobChan {
				res, err := e.processBatch(ctx, llm, job.entries)
				resultChan <- batchResult{index: job.index, results: res, err: err}
			}
		}(w)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// ── Collect and write in order ──
	type stored struct {
		results []EnrichmentResult
		err     error
	}
	pending := make(map[int]*stored)
	nextIndex := 0
	totalResults := 0

	for res := range resultChan {
		if res.err != nil {
			failed.Add(1)
			pending[res.index] = &stored{err: res.err}
			log.Printf("Batch %d failed: %v", res.index, res.err)
		} else {
			pending[res.index] = &stored{results: res.results}
			totalResults += len(res.results)
		}
		processed.Add(1)

		for {
			s, ok := pending[nextIndex]
			if !ok {
				break
			}
			if s.err == nil && len(s.results) > 0 {
				if err := InsertEnrichmentResults(targetDB, s.results, batches[nextIndex]); err != nil {
					log.Printf("Failed to insert batch %d: %v", nextIndex, err)
					failed.Add(1)
				}
			}
			delete(pending, nextIndex)
			nextIndex++
		}
	}

	elapsed := time.Since(startTime).Round(time.Second)
	log.Printf("Done! processed:%d failed:%d entries:%d elapsed:%s",
		processed.Load(), failed.Load(), totalResults, elapsed)
	return nil
}

// processBatch sends one batch of entries to the LLM for analysis.
func (e *Enricher) processBatch(ctx context.Context, llm *LLMClient, batch []EnrichmentEntry) ([]EnrichmentResult, error) {
	// Build a compact representation for the LLM
	type entryBrief struct {
		ID       int    `json:"id"`
		Headword string `json:"headword"`
		Pinyin   string `json:"pinyin"`
		Meanings string `json:"meanings"`
	}
	briefs := make([]entryBrief, len(batch))
	for i, en := range batch {
		briefs[i] = entryBrief{
			ID:       en.EntryID,
			Headword: en.Headword,
			Pinyin:   en.Pinyin,
			Meanings: strings.Join(en.Meanings, " | "),
		}
	}

	userPrompt := fmt.Sprintf(
		`Проанализируй записи китайско-русского словаря (БКРС). Для каждой записи:

1. **pinyin_issues** — что не так с пиньинем (пусто, ошибка тона, лишние символы, мусор, "ok")
2. **pinyin_fixed** — исправленный пиньинь (если issues не "ok", иначе пустая строка)
3. **meaning_issues** — что не так со значением (слишком короткое, мусор, тавтология, см. ссылку, орфография, "ok")
4. **meaning_fixed** — улучшенное/дополненное значение (если issues не "ok", иначе пустая строка)
5. **meaning_quality** — оценка качества от 1 до 10
6. **needs_review** — true если есть серьёзные проблемы, требующие ручной проверки

Входные данные (JSON):
%s

Формат ответа — строго JSON-массив:
[
  {"entry_id": 1, "pinyin_issues": "ok", "pinyin_fixed": "", "meaning_issues": "ok", "meaning_fixed": "", "meaning_quality": 8, "needs_review": false},
  ...
]

Только JSON, без пояснений.`, toJSON(briefs))

	systemPrompt := `Ты — лексикограф-синолог, эксперт по китайскому языку и словарю БКРС (Большой китайско-русский словарь). Анализируешь словарные записи: проверяешь пиньинь, качество перевода, выявляешь мусор и ошибки. Отвечай строго в формате JSON.`

	var results []EnrichmentResult
	if err := llm.ChatJSON(ctx, systemPrompt, userPrompt, 0.3, 4096, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// toJSON is a simple helper to marshal any value to a JSON string.
func toJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`[{"error":%q}]`, err)
	}
	return string(b)
}

// reportProgress logs progress periodically.
func reportProgress(ctx context.Context, label string, processed, failed *atomic.Int64, total int, startTime time.Time) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			done := processed.Load()
			f := failed.Load()
			elapsed := time.Since(startTime).Round(time.Second)
			pct := float64(done) * 100 / float64(total)
			rate := 0.0
			if elapsed.Seconds() > 0 {
				rate = float64(done) / elapsed.Seconds()
			}
			eta := "?"
			if rate > 0 {
				eta = (time.Duration(float64(int64(total)-done)/rate) * time.Second).Round(time.Second).String()
			}
			log.Printf("[%s] %d/%d (%.1f%%) failed:%d elapsed:%s rate:%.1f/s ETA:%s",
				label, done, total, pct, f, elapsed, rate, eta)
		}
	}
}
