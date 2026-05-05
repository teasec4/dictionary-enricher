package main

import (
	"flag"
	"fmt"
	"log"

	"dabkrs-examples/internal/infrastructure/database"
	"dabkrs-examples/internal/repository"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	dbPath := flag.String("db", "newDB.db", "path to DB")
	headword := flag.String("headword", "", "search by headword")
	limit := flag.Int("limit", 10, "limit entries")
	flag.Parse()

	db, err := database.OpenSourceDB(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := repository.NewDBRepo(db)

	if *headword != "" {
		examples, err := repo.GetExamples(*limit)
		if err != nil {
			log.Fatalf("get: %v", err)
		}
		for _, ex := range examples {
			if ex.Headword == *headword {
				fmt.Printf("Found: %s -> %s\n", ex.Headword, ex.Text)
			}
		}
		return
	}

	examples, err := repo.GetExamples(*limit)
	if err != nil {
		log.Fatalf("get examples: %v", err)
	}

	fmt.Printf("First %d examples:\n", len(examples))
	for _, e := range examples {
		fmt.Printf("  %s: %s\n", e.Headword, e.Text)
	}
}