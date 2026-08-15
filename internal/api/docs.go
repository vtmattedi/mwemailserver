package api

import (
	"net/http"

	"github.com/vtmattedi/mw-email-server/internal/apidocs"
)

type adminDocsListResponse struct {
	Versions []apidocs.Version `json:"versions"`
}

type adminDocResponse struct {
	Version       string `json:"version"`
	Status        string `json:"status"`
	ContentType   string `json:"content_type"`
	Documentation string `json:"documentation"`
}

func AdminListAPIDocs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, adminDocsListResponse{
		Versions: apidocs.List(),
	})
}

func AdminGetAPIDocs(w http.ResponseWriter, r *http.Request) {
	meta, body, err := apidocs.Get(r.PathValue("version"))
	if err != nil {
		writeError(w, http.StatusNotFound, "API documentation version not found")
		return
	}

	w.Header().Set("X-MW-API-Docs-Version", meta.Version)
	writeJSON(w, http.StatusOK, adminDocResponse{
		Version:       meta.Version,
		Status:        meta.Status,
		ContentType:   meta.ContentType,
		Documentation: body,
	})
}
