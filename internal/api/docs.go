package api

import (
	"net/http"

	"github.com/vtmattedi/mw-email-server/internal/apidocs"
)

func AdminListAPIDocs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"versions": apidocs.List(),
	})
}

func AdminGetAPIDocs(w http.ResponseWriter, r *http.Request) {
	meta, body, err := apidocs.Get(r.PathValue("version"))
	if err != nil {
		writeError(w, http.StatusNotFound, "API documentation version not found")
		return
	}

	w.Header().Set("Content-Type", meta.ContentType+"; charset=utf-8")
	w.Header().Set("X-MW-API-Docs-Version", meta.Version)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}
