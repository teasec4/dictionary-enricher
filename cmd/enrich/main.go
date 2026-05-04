package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"dabkrs-examples/internal/config"
	"dabkrs-examples/internal/infrastructure/database"
	"dabkrs-examples/internal/infrastructure/llm"
	"dabkrs-examples/internal/repository"
	"dabkrs-examples/internal/usecase"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	dbPath := flag.String("db", "newDictionary.db", "path to target DB to write enriched data")
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

	// Open target DB
	log.Printf("Opening target DB: %s", *dbPath)
	tgtDB, err := database.CreateNewDB(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open target DB: %v\n", err)
		os.Exit(1)
	}
	defer tgtDB.Close()

	// Init repositories
	exampleRepo := repository.NewExampleRepo(tgtDB)
	

	// Init LLM client
	llmClient := llm.NewClient(llm.Config{
		BaseURL: cfg.LLMBaseURL,
		Model:   cfg.LLMModel,
	})

	
}
