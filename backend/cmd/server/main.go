package main

import (
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
	log.Printf("xiaopi api listening on :%s", port)
	if err := http.ListenAndServe(":"+port, httpapi.NewServer(mysqlStore).Handler()); err != nil {
		log.Fatal(err)
	}
}
