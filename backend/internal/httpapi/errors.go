package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
)

// errorBody is the uniform error shape of the whole API: every 4xx and 5xx
// response carries exactly these two fields (AC-19).
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON serialises v as JSON with the given status. It never logs the
// payload — response bodies can carry customer data.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("httpapi: failed to encode response body: %v", err)
	}
}

// writeError writes the uniform error body.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Code: code, Message: message})
}

// writeNotImplemented answers a route whose owner ticket has not merged yet.
// It is a 501, never a 500: a 500 means the server crashed on a request it was
// supposed to serve.
func writeNotImplemented(w http.ResponseWriter, method, path string) {
	writeError(w, http.StatusNotImplemented, "not_implemented",
		method+" "+path+" is not implemented yet")
}
