package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"dabkrs-examples/internal/config"
	"dabkrs-examples/internal/domain"
	"dabkrs-examples/internal/infrastructure/database"
	"dabkrs-examples/internal/infrastructure/llm"
	"dabkrs-examples/internal/repository"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	dbPath := flag.String("db", "dictionary.db", "path to source DB with entries")
	llmURL := flag.String("llm", "", "LLM API base URL (overrides LLM_BASE_URL env)")
	model := flag.String("model", "", "LLM model (overrides LLM_MODEL env)")
	limit := flag.Int("limit", 0, "max entries to process (0 = all)")
	batch := flag.Int("batch", 20, "entries per batch for DB pagination and clean")
	exampleBatch := flag.Int("example-batch", 3, "entries per batch for examples (smaller = less thinking tokens)")
	maxChars := flag.Int("max-chars", 2, "max headword rune count to enrich")
	flag.Parse()

	// Load config from .env + flags override
	cfg := config.Load()
	if *llmURL != "" {
		cfg.LLMBaseURL = *llmURL
	}
	if *model != "" {
		cfg.LLMModel = *model
	}
	if *limit > 0 {
		cfg.Limit = *limit
	}
	if *batch > 0 {
		cfg.BatchSize = *batch
	}
	if *exampleBatch > 0 {
		cfg.ExampleBatchSize = *exampleBatch
	}
	if *maxChars > 0 {
		cfg.MaxChars = *maxChars
	}

	// Graceful shutdown context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	const newDBpath string = "./newDB.db"
	// Open New DB
	log.Printf("Opening target DB: %s", newDBpath)
	tgtDB, err := database.CreateNewDB(newDBpath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open target DB: %v\n", err)
		os.Exit(1)
	}
	defer tgtDB.Close()

	// Open Source DB
	srcDB, err := database.OpenSourceDB(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open src DB: %v\n", err)
		os.Exit(1)
	}
	defer srcDB.Close()

	// Init repositories
	dbRepo := repository.NewDBRepo(srcDB)
	newDBrepo := repository.NewDBRepo(tgtDB) // newDBRepo for later

	// Init LLM client
	llmClient := llm.NewClient(llm.Config{
		BaseURL: cfg.LLMBaseURL,
		Model:   cfg.LLMModel,
	})

	headwordsCh := make(chan string, cfg.BatchSize)
	examplesCh := make(chan domain.Example, cfg.BatchSize)

	var wg sync.WaitGroup
	wg.Add(1)

	type Content struct {
		Examples []domain.Example
	}

	// WRITER - async bulk insert
	go func() {
		defer wg.Done()
		var buf []domain.Example
		flush := func() {
			if len(buf) > 0 {
				if err := newDBrepo.BulkInsert(ctx, buf); err != nil {
					log.Printf("bulk insert error: %v", err)
				} else {
					log.Printf("bulk inserted %d examples", len(buf))
				}
				buf = buf[:0]
			}
		}

		for {
			select {
			case ex, ok := <-examplesCh:
				if !ok {
					flush()
					return
				}
				buf = append(buf, ex)
				if len(buf) >= cfg.BatchSize {
					flush()
				}
			case <-ctx.Done():
				flush()
				return
			}
		}
	}()

	// PRODUCER - running in background
	go func() {
		log.Println("Producer: starting...")
		lastID := 0
		loaded := 0
		skipped := 0
		for {
			headwords, err := dbRepo.LoadHeadwords(ctx, cfg.BatchSize, lastID)
			if err != nil {
				log.Printf("load error: %v", err)
				break
			}
			if len(headwords) == 0 {
				break
			}
			lastID = headwords[len(headwords)-1].ID
			loaded += len(headwords)

			for _, hw := range headwords {
				if len([]rune(hw.Headword)) > cfg.MaxChars {
					skipped++
				continue
				}
				select {
				case headwordsCh <- hw.Headword:
				case <-ctx.Done():
					close(headwordsCh)
					return
				}
			}
		}
		close(headwordsCh)
		log.Printf("Producer done! Sent %d headwords, skipped %d (max %d chars)", loaded, skipped, cfg.MaxChars)
	}()

	// CONSUMER - LLM processing
	log.Println("Consumer: starting LLM processing...")
	processed := 0
	consumerLoop:
	for {
		var batch []string
		for i := 0; i < cfg.BatchSize; i++ {
			select {
			case hw, ok := <-headwordsCh:
				if !ok {
					break consumerLoop
				}
				batch = append(batch, hw)
			case <-ctx.Done():
				return
			}
		}

		if len(batch) == 0 {
			break
		}

		userPrompt := strings.Join(batch, ", ")
		log.Printf("Sending to LLM: %s", userPrompt)

		var resp Content
		if err := llmClient.ChatJSON(ctx, userPrompt, &resp); err != nil {
			log.Printf("LLM error: %v", err)
			continue
		}

		processed += len(resp.Examples)
		log.Printf("Received %d examples (total: %d)", len(resp.Examples), processed)
		for _, ex := range resp.Examples {
			log.Printf("  %s: %s", ex.Headword, ex.Text)
			examplesCh <- ex
		}
	}

	close(examplesCh)
	log.Printf("Waiting for writer...")

	wg.Wait()
	log.Printf("Done! Total examples: %d", processed)
}
