package api

import (
	"encoding/json"
	"log"
	"net/http"
)

type drawResponse struct {
	PrizeID int    `json:"prize_id"`
	Name    string `json:"name"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: %v", err)
	}

}
