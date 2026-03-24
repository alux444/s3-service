package httpapi

import (
	"net/http"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeOK(w, r, map[string]string{"status": "ok"})
}
