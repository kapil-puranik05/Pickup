package main

import (
	"log"
	"net/http"
	"router/internal/database"
	"router/internal/handlers"
)

func main() {
	database.ConnectDB()
	log.Println("Database connected successfully")
	handlers.Init()
	mux := http.NewServeMux()
	mux.HandleFunc("/register", handlers.ChainRegistrationHandler)
	mux.HandleFunc("/upload", handlers.UploadInitializationHandler)
	mux.HandleFunc("/upload-complete", handlers.UploadCompleteHandler)
	mux.HandleFunc("/retrieve", handlers.RetrievalInitializationHandler)
	mux.HandleFunc("/delete", handlers.DeleteInitializationHandler)
	mux.HandleFunc("/delete-complete", handlers.DeleteCompleteHandler)
	server := &http.Server{
		Addr:    "localhost:8000",
		Handler: mux,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Panic(err)
	}
}
