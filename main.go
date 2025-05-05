package main

import (
	"log"
	"net/http"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found or could not be loaded")
	}

	// Initialize the PostgreSQL database
	if err := initDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	log.Println("Connected to PostgreSQL database successfully")

	// Set up the router with Gorilla Mux
	router := setupRouter()

	// Start the server on port 8080
	log.Println("Animation Server starting on port 8080...")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
