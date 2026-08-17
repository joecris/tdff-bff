package handlers

import (
	"encoding/json"
	"log"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("handlers: write response: %v", err)
	}
}

// clientError writes a generic message to the client while logging the real
// cause server-side — callback/exchange failures can carry details (Auth0
// error descriptions, token endpoint responses) that shouldn't reach the
// browser per the BFF's "no internals leak past the boundary" posture.
func clientError(w http.ResponseWriter, status int, publicMsg string, logMsg string, err error) {
	log.Printf("handlers: %s: %v", logMsg, err)
	http.Error(w, publicMsg, status)
}
