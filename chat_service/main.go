package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tarantool/go-tarantool"
)

// Define Prometheus metrics based on the RED principle
var (
	requestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chat_service_requests_total",
			Help: "Total number of requests to the chat service",
		},
		[]string{"endpoint", "method"},
	)

	errorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chat_service_errors_total",
			Help: "Total number of errors in the chat service",
		},
		[]string{"endpoint", "method"},
	)

	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chat_service_request_duration_seconds",
			Help:    "Histogram of request durations for the chat service",
			Buckets: prometheus.DefBuckets, // Default buckets provided by Prometheus
		},
		[]string{"endpoint", "method"},
	)
)

var conn *tarantool.Connection

// Initialize connection to Tarantool
func init() {
	var err error
	conn, err = tarantool.Connect("tarantool:3301", tarantool.Opts{
		User: "guest",
	})
	if err != nil {
		log.Fatalf("Connection to Tarantool failed: %s", err)
	}
	prometheus.MustRegister(requestCounter)
	prometheus.MustRegister(errorCounter)
	prometheus.MustRegister(requestDuration)
}

type DialogRequest struct {
	GetterId string `json:"getter_id"`
	Text     string `json:"text"`
}

func convertMapI2S(data interface{}) interface{} {
	switch v := data.(type) {
	case map[interface{}]interface{}:
		newMap := make(map[string]interface{})
		for key, value := range v {
			strKey := fmt.Sprintf("%v", key)      // Convert key to string
			newMap[strKey] = convertMapI2S(value) // Recursively convert values
		}
		return newMap
	case []interface{}:
		for i, value := range v {
			v[i] = convertMapI2S(value) // Recursively convert elements in slices
		}
	}
	return data
}

func dialogSend(w http.ResponseWriter, r *http.Request) {
	method := r.Method
	endpoint := "/dialog/send"
	var dialog DialogRequest
	if err := json.NewDecoder(r.Body).Decode(&dialog); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Use a dummy current_user for now
	currentUser := r.Header.Get("X-User-Id")

	_, err := conn.Call("dialog_send", []interface{}{currentUser, dialog.GetterId, dialog.Text})
	if err != nil {
		errorCounter.WithLabelValues(endpoint, method).Inc()
		http.Error(w, "Error adding dialog to database", http.StatusInternalServerError)
		return
	}
	// Define the request body
	requestBody := map[string]int64{
		"increment": 1, // Change this value to adjust how much to increment
	}

	// Convert the request body to JSON
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		errorCounter.WithLabelValues(endpoint, method).Inc()
		fmt.Printf("Error encoding JSON: %v\n", err)
		return
	}
	http.Post("http://counter-service:8080/counters?user_id="+currentUser, "application/json", bytes.NewBuffer(jsonBody))
	fmt.Fprintln(w, "Sent")
}

func dialogList(w http.ResponseWriter, r *http.Request) {
	method := r.Method
	endpoint := "/dialog/list"
	currentUser := r.Header.Get("X-User-Id")
	recipientID := r.URL.Query().Get("recipient_id")
	fmt.Println(r)
	if recipientID == "" {
		errorCounter.WithLabelValues(endpoint, method).Inc()
		http.Error(w, "recipient_id is required", http.StatusBadRequest)
		return
	}

	resp, err := conn.Call("dialog_list", []interface{}{currentUser, recipientID})
	if err != nil {
		errorCounter.WithLabelValues(endpoint, method).Inc()
		http.Error(w, "Error getting dialogs", http.StatusInternalServerError)
		return
	}

	dialogs, err := json.Marshal(convertMapI2S(resp.Data))
	if err != nil {
		errorCounter.WithLabelValues(endpoint, method).Inc()
		http.Error(w, "Error encoding dialogs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, string(dialogs))
}

func main() {
	// router := mux.NewRouter()

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/dialog/send", instrument(dialogSend, "/dialog/send"))
	http.HandleFunc("/dialog/list", instrument(dialogList, "/dialog/list"))

	log.Fatal(http.ListenAndServe(":8081", nil))
}

// instrument is a wrapper function to add Prometheus monitoring to an HTTP handler.
func instrument(next http.HandlerFunc, endpoint string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		method := r.Method

		// Increment the request rate counter
		requestCounter.WithLabelValues(endpoint, method).Inc()

		// Call the actual handler
		next(w, r)

		// Measure the duration of the request
		duration := time.Since(start).Seconds()
		requestDuration.WithLabelValues(endpoint, method).Observe(duration)
	}
}
