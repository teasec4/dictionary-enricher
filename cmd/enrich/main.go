package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"dabkrs-examples/internal/config"
	"dabkrs-examples/internal/service"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// sourcePath := flag.String("source", "../backend/dictionary.db", "path to source dictionary.db")
	// targetPath := flag.String("target", "enrichments.db", "path to enrichments output DB")
	// llmBaseURL := flag.String("llm", "http://localhost:1234", "LLM API base URL")
	// llmModel := flag.String("model", "", "LLM model (auto)")
	// workers := flag.Int("workers", 5, "concurrent workers")
	// batchSize := flag.Int("batch", 20, "entries per LLM call")
	// limit := flag.Int("limit", 0, "max entries to process (0 = all)")
	flag.Parse()

	cfg := config.Load()

	newBD, err := service.CreateNewDB("newDictionary.db")
	if err != nil {
		fmt.Println(err)
	}
	log.Println("created new database")
	
	oldDb, err := service.Open("sqlite3", cfg.DbPath)
	if err != nil {
		fmt.Println(err)
	}
	log.Println("opened old database")

	// close db
	defer newBD.Close()
	defer oldDb.Close()
	
	dbRepo := service.NewRepo(oldDb)
	log.Println("init old database repo")
	newDbRepo := service.NewRepo(newBD)
	log.Println("init new database repo")

	// createing context
	ctx := context.Background()
	
	total, err := dbRepo.Count(ctx)
	if err != nil {
	    log.Fatal(err)
	}

	const batchSize = 20
	const batchMax = 3
	batchCount := 0
	
	for offset := 0; offset < total; offset += batchSize {
		if batchCount >= batchMax{
			break
		}

		entries, err := dbRepo.List(ctx, batchSize, offset)
	    if err != nil {
	        log.Fatal(err)
	    }
	
	    // тут обрабатываешь entries (LLM, enrich и т.д.)
	
	    for _, e := range entries {
	        if err := newDbRepo.Create(ctx, &e); err != nil {
	            log.Fatal(err)
	        }
	    }
	
	    log.Printf("processed %d / %d entries", offset+len(entries), total)

		batchCount++
	}

	
	// log.Printf("Starting dictionary enrichment: limit=%d workers=%d batch=%d",
	// 	cfg.Limit, cfg.Workers, cfg.BatchSize)
}
