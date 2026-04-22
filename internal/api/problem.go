package api

import (
	"encoding/json"
	"log"
	"net/http"
)

// redactedServerDetail is returned for any HTTP 5xx Problem response body so operators
// rely on server logs (request id + logged detail) instead of leaking internals to clients.
const redactedServerDetail = "An unexpected error occurred. Try again later or contact the operator if it persists."

type Problem struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
	Code     string `json:"code,omitempty"`
}

func WriteProblem(w http.ResponseWriter, r *http.Request, status int, title, detail, code string) {
	outDetail := detail
	if status >= http.StatusInternalServerError {
		rid, _ := r.Context().Value(requestIDKey).(string)
		log.Printf("problem status=%d code=%q request_id=%s detail=%q", status, code, rid, detail)
		outDetail = redactedServerDetail
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		rid, _ := r.Context().Value(requestIDKey).(string)
		log.Printf("http %s %s -> %d code=%q detail=%q request_id=%s",
			r.Method, r.URL.Path, status, code, outDetail, rid)
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Problem{
		Title:  title,
		Status: status,
		Detail: outDetail,
		Code:   code,
	})
}
