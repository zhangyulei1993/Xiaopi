package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"xiaopi/backend/internal/httpapi"
	"xiaopi/backend/internal/store"
)

func main() {
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	mysqlStore, err := store.OpenMySQL()
	if err != nil {
		log.Fatal(err)
	}
	defer mysqlStore.Close()
	if err := store.ApplyMigrations(mysqlStore.DB(), "migrations"); err != nil {
		log.Fatal(err)
	}
	log.Printf("xiaopi api listening on :%s", port)
	server := httpapi.NewServer(mysqlStore)
	go server.RunStagedMediaCleanup(context.Background())
	go server.RunSubscriptionOutbox(context.Background())
	go server.RunCommunitySync(context.Background())
	if err := http.ListenAndServe(":"+port, server.Handler()); err != nil {
		log.Fatal(err)
	}
}
