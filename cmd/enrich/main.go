package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"dabkrs-examples/internal"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	sourcePath := flag.String("source", "../backend/dictionary.db", "path to source dictionary.db")
	targetPath := flag.String("target", "enrichments.db", "path to enrichments output DB")
	llmBaseURL := flag.String("llm", "http://localhost:1234", "LLM API base URL")
	llmModel := flag.String("model", "", "LLM model (auto)")
	workers := flag.Int("workers", 5, "concurrent workers")
	batchSize := flag.Int("batch", 20, "entries per LLM call")
	limit := flag.Int("limit", 0, "max entries to process (0 = all)")
	flag.Parse()

	model := *llmModel
	if model == "" {
		model = "gemma-4"
	}

	cfg := internal.EnricherConfig{
		SourcePath: *sourcePath,
		TargetPath: *targetPath,
		LLMBaseURL: *llmBaseURL,
		LLMModel:   model,
		Workers:    *workers,
		BatchSize:  *batchSize,
		Limit:      *limit,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down gracefully...")
		cancel()
	}()

	log.Printf("Starting dictionary enrichment: limit=%d workers=%d batch=%d",
		cfg.Limit, cfg.Workers, cfg.BatchSize)

	enricher := internal.NewEnricher(cfg)
	if err := enricher.Run(ctx); err != nil {
		log.Fatalf("Enrichment failed: %v", err)
	}
}
