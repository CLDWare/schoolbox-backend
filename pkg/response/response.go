package response

import (
	"encoding/json"
	"net/http"
	"time"
)

// Response represents a standard API response
type Response struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message,omitempty"`
	Data      any       `json:"data,omitempty"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// JSON writes a JSON response
func JSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := Response{
		Success:   statusCode < 400,
		Data:      data,
		Timestamp: time.Now(),
	}

	json.NewEncoder(w).Encode(response)
}

// Error writes an error JSON response
func Error(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := Response{
		Success: false,
		Error:   message,
	}

	json.NewEncoder(w).Encode(response)
}

// Success writes a success JSON response
func Success(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, data)
}

// Created writes a created JSON response
func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, data)
}
