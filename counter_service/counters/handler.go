package counters

import (
	"encoding/json"
	"net/http"
)

type CounterHandler struct {
	service *CounterService
}

func NewCounterHandler(service *CounterService) *CounterHandler {
	return &CounterHandler{service: service}
}

func (h *CounterHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	count, err := h.service.GetUnreadCount(userID)
	if err != nil {
		http.Error(w, "failed to get unread count", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":      userID,
		"unread_count": count,
	})
}

func (h *CounterHandler) IncrementUnreadCount(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	var req struct {
		Increment int64 `json:"increment"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.service.IncrementUnreadCount(userID, req.Increment)
	if err != nil {
		http.Error(w, "failed to increment unread count", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
