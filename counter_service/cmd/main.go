package main

import (
	"counter-service/cache"
	"counter-service/counters"
	"counter-service/storage"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	// Initialize Redis cache
	redisCache := cache.NewRedisCache("redis:6379", "", 0)

	// Initialize DB
	db := storage.NewDB("host=haproxy port=5432 user=postgres password=pass dbname=postgres sslmode=disable")

	// Initialize counter service
	counterService := counters.NewCounterService(redisCache, db)

	// Initialize HTTP handler
	counterHandler := counters.NewCounterHandler(counterService)

	// Setup router
	r := mux.NewRouter()
	r.HandleFunc("/counters", counterHandler.GetUnreadCount).Methods("GET")
	r.HandleFunc("/counters", counterHandler.IncrementUnreadCount).Methods("POST")

	// Start server
	log.Println("Counter service is running on port 8080...")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
