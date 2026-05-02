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
	// CLI flags
	sourcePath := flag.String("source", "../backend/dictionary.db", "path to source dictionary.db")
	targetPath := flag.String("target", "examples.db", "path to target examples.db")
	llmBaseURL := flag.String("llm", "http://localhost:1234", "LLM API base URL")
	llmModel := flag.String("model", "", "LLM model name (auto-detected if empty)")
	workers := flag.Int("workers", 5, "number of concurrent workers")
	batchSize := flag.Int("batch", 50, "words per LLM call")
	limit := flag.Int("limit", 0, "max words to process (0 = all)")

	flag.Parse()

	// Resolve model if not provided
	model := *llmModel
	if model == "" {
		model = "gemma-4"
		log.Printf("No model specified, defaulting to: %s", model)
	}

	cfg := internal.PipelineConfig{
		SourcePath: *sourcePath,
		TargetPath: *targetPath,
		LLMBaseURL: *llmBaseURL,
		LLMModel:   model,
		Workers:    *workers,
		BatchSize:  *batchSize,
		Limit:      *limit,
	}

	// Context with graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down gracefully...", sig)
		cancel()
	}()

	pipeline := internal.NewPipeline(cfg)
	if err := pipeline.Run(ctx); err != nil {
		log.Fatalf("Pipeline failed: %v", err)
	}
}
