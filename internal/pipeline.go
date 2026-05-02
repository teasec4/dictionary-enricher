package internal

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// PipelineConfig holds configuration for the example generation pipeline.
type PipelineConfig struct {
	SourcePath string // path to dictionary.db
	TargetPath string // path to examples.db
	LLMBaseURL string
	LLMModel   string
	Workers    int  // number of concurrent workers
	BatchSize  int  // words per LLM call
	Limit      int  // max words to process (0 = all)
}

// Pipeline orchestrates the full example generation pipeline.
type Pipeline struct {
	cfg PipelineConfig
}

// NewPipeline creates a new pipeline with the given configuration.
func NewPipeline(cfg PipelineConfig) *Pipeline {
	if cfg.Workers <= 0 {
		cfg.Workers = 5
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}
	return &Pipeline{cfg: cfg}
}

// Run executes the pipeline: read source, process batches, write results.
func (p *Pipeline) Run(ctx context.Context) error {
	log.Printf("Opening source DB: %s", p.cfg.SourcePath)
	sourceDB, err := sql.Open("sqlite3", p.cfg.SourcePath+"?mode=ro")
	if err != nil {
		return fmt.Errorf("open source DB: %w", err)
	}
	defer sourceDB.Close()

	log.Printf("Opening target DB: %s", p.cfg.TargetPath)
	targetDB, err := sql.Open("sqlite3", p.cfg.TargetPath)
	if err != nil {
		return fmt.Errorf("open target DB: %w", err)
	}
	defer targetDB.Close()

	if err := InitTargetDB(targetDB); err != nil {
		return fmt.Errorf("init target DB: %w", err)
	}

	log.Println("Reading already processed headwords...")
	known, err := ReadKnownHeadwords(targetDB)
	if err != nil {
		return fmt.Errorf("read known headwords: %w", err)
	}
	log.Printf("Already processed: %d headwords", len(known))

	log.Println("Reading source words...")
	words, err := ReadWords(sourceDB, known, p.cfg.Limit)
	if err != nil {
		return fmt.Errorf("read source words: %w", err)
	}
	log.Printf("Source words to process: %d", len(words))

	if len(words) == 0 {
		log.Println("Nothing to process.")
		return nil
	}

	llm := NewLLMClient(LLMConfig{
		BaseURL: p.cfg.LLMBaseURL,
		Model:   p.cfg.LLMModel,
	})

	// Split into batches
	batches := make([][]string, 0, (len(words)+p.cfg.BatchSize-1)/p.cfg.BatchSize)
	for i := 0; i < len(words); i += p.cfg.BatchSize {
		end := i + p.cfg.BatchSize
		if end > len(words) {
			end = len(words)
		}
		batches = append(batches, words[i:end])
	}
	log.Printf("Total batches: %d (batch size: %d)", len(batches), p.cfg.BatchSize)

	// Progress tracking
	var processed atomic.Int64
	var failed atomic.Int64
	startTime := time.Now()
	total := len(batches)

	// Progress reporter
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				done := processed.Load()
				fail := failed.Load()
				elapsed := time.Since(startTime).Round(time.Second)
				pct := float64(done) * 100 / float64(total)
				rate := 0.0
				if elapsed.Seconds() > 0 {
					rate = float64(done) / elapsed.Seconds()
				}
				eta := "?"
				if rate > 0 {
					remaining := time.Duration(float64(int64(total)-done)/rate) * time.Second
					eta = remaining.Round(time.Second).String()
				}
				log.Printf("Progress: %d/%d batches (%.1f%%) | failed: %d | elapsed: %s | rate: %.1f batches/s | ETA: %s",
					done, total, pct, fail, elapsed, rate, eta)
			}
		}
	}()

	// Worker pool
	type batchJob struct {
		index  int
		words  []string
	}

	type batchResult struct {
		index  int
		results []ExampleResult
		err    error
	}

	jobChan := make(chan batchJob, len(batches))
	resultChan := make(chan batchResult, len(batches))

	// Queue jobs
	go func() {
		for i, batch := range batches {
			jobChan <- batchJob{index: i, words: batch}
		}
		close(jobChan)
	}()

	// Start workers
	var wg sync.WaitGroup
	for w := 0; w < p.cfg.Workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobChan {
				results, err := llm.GenerateExamplesBatch(ctx, job.words)
				resultChan <- batchResult{index: job.index, results: results, err: err}
			}
		}(w)
	}

	// Close result channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect and write results in order
	results := make(map[int][]ExampleResult)
	errors := make(map[int]error)
	nextIndex := 0
	totalResults := 0

	for result := range resultChan {
		if result.err != nil {
			failed.Add(1)
			errors[result.index] = result.err
			log.Printf("Batch %d failed: %v", result.index, result.err)
		} else {
			results[result.index] = result.results
			totalResults += len(result.results)
		}
		processed.Add(1)

		// Flush sequential results
		for {
			if res, ok := results[nextIndex]; ok {
				if err := InsertExamples(targetDB, res); err != nil {
					log.Printf("Failed to insert batch %d: %v", nextIndex, err)
					failed.Add(1)
				}
				delete(results, nextIndex)
			} else if _, ok := errors[nextIndex]; ok {
				delete(errors, nextIndex)
			} else {
				break
			}
			nextIndex++
		}
	}

	elapsed := time.Since(startTime).Round(time.Second)
	fail := failed.Load()
	log.Printf("Done! Processed %d batches, failed: %d, total examples: %d, elapsed: %s",
		processed.Load(), fail, totalResults, elapsed)

	return nil
}
