package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/tarantool/go-tarantool"
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
	var dialog DialogRequest
	if err := json.NewDecoder(r.Body).Decode(&dialog); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Use a dummy current_user for now
	currentUser := r.Header.Get("X-User-Id")

	_, err := conn.Call("dialog_send", []interface{}{currentUser, dialog.GetterId, dialog.Text})
	if err != nil {
		http.Error(w, "Error adding dialog to database", http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, "Sent")
}

func dialogList(w http.ResponseWriter, r *http.Request) {
	currentUser := r.Header.Get("X-User-Id")
	recipientID := r.URL.Query().Get("recipient_id")
	fmt.Println(r)
	if recipientID == "" {
		http.Error(w, "recipient_id is required", http.StatusBadRequest)
		return
	}

	resp, err := conn.Call("dialog_list", []interface{}{currentUser, recipientID})
	if err != nil {
		http.Error(w, "Error getting dialogs", http.StatusInternalServerError)
		return
	}

	dialogs, err := json.Marshal(convertMapI2S(resp.Data))
	if err != nil {
		http.Error(w, "Error encoding dialogs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, string(dialogs))
}

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/dialog/send", dialogSend).Methods("POST")
	router.HandleFunc("/dialog/list", dialogList).Methods("GET")

	log.Fatal(http.ListenAndServe(":8081", router))
}
