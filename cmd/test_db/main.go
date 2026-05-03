package main

import (
	"context"
	"dabkrs-examples/internal/service"
	"fmt"
	"log"
)

func main(){
	// open db
	db, err := service.Open("sqlite3", "newDictionary.db")
	if err != nil {
	    log.Fatal(err)
	}

	dbRepo := service.NewRepo(db)
	ctx := context.Background()

	enties, err := dbRepo.List(ctx, 10, 0)

	fmt.Println(enties)
}