package handlers

import (
	"net/http"

	"s3-service/internal/httpapi"
)

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteOK(w, r, map[string]string{"status": "ok"})
}
