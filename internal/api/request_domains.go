// MW_ALLOWED_REQUEST_DOMAINS_V1
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/vtmattedi/mw-email-server/internal/requestorigin"
)

type RequestDomainStore interface {
	GetRequestDomainRuleForToken(
		ctx context.Context,
		rawToken string,
	) (projectID int64, rule string, err error)

	GetProjectRequestDomainRule(
		ctx context.Context,
		projectID int64,
	) (string, error)

	SetProjectRequestDomainRule(
		ctx context.Context,
		projectID int64,
		rule string,
	) error
}

type requestDomainRuleResponse struct {
	ProjectID             int64  `json:"project_id"`
	AllowedRequestDomains string `json:"allowed_request_domains"`
}

type requestDomainRuleRequest struct {
	AllowedRequestDomains string `json:"allowed_request_domains"`
}

func RequireAllowedRequestDomain(
	store RequestDomainStore,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawToken := bearerToken(r.Header.Get("Authorization"))
		if rawToken == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		_, rule, err := store.GetRequestDomainRuleForToken(
			r.Context(),
			rawToken,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			writeError(
				w,
				http.StatusInternalServerError,
				"could not verify request origin",
			)
			return
		}

		rule, err = requestorigin.NormalizeRule(rule)
		if err != nil {
			writeError(
				w,
				http.StatusInternalServerError,
				"project request-domain configuration is invalid",
			)
			return
		}

		if rule == "*" {
			next.ServeHTTP(w, r)
			return
		}

		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			writeError(
				w,
				http.StatusForbidden,
				"origin header required for this api key",
			)
			return
		}

		hostname, err := requestorigin.OriginHostname(origin)
		if err != nil {
			writeError(w, http.StatusForbidden, "request origin is not allowed")
			return
		}

		allowed, err := requestorigin.Allowed(rule, hostname)
		if err != nil {
			writeError(
				w,
				http.StatusInternalServerError,
				"project request-domain configuration is invalid",
			)
			return
		}

		if !allowed {
			writeError(w, http.StatusForbidden, "request origin is not allowed")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func RequestEmailCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/emails" {
			next.ServeHTTP(w, r)
			return
		}

		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")

		if r.Method == http.MethodOptions {
			w.Header().Set(
				"Access-Control-Allow-Methods",
				"POST, OPTIONS",
			)
			w.Header().Set(
				"Access-Control-Allow-Headers",
				"Authorization, Content-Type, Idempotency-Key",
			)
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func AdminGetProjectRequestDomains(
	store RequestDomainStore,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID, err := parseRequestDomainProjectID(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid project id")
			return
		}

		rule, err := store.GetProjectRequestDomainRule(
			r.Context(),
			projectID,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}

			writeError(
				w,
				http.StatusInternalServerError,
				"could not load allowed request domains",
			)
			return
		}

		writeJSON(
			w,
			http.StatusOK,
			requestDomainRuleResponse{
				ProjectID:             projectID,
				AllowedRequestDomains: rule,
			},
		)
	}
}

func AdminPutProjectRequestDomains(
	store RequestDomainStore,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID, err := parseRequestDomainProjectID(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid project id")
			return
		}

		var input requestDomainRuleRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		if err := decoder.Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		rule, err := requestorigin.NormalizeRule(
			input.AllowedRequestDomains,
		)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := store.SetProjectRequestDomainRule(
			r.Context(),
			projectID,
			rule,
		); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}

			writeError(
				w,
				http.StatusInternalServerError,
				"could not update allowed request domains",
			)
			return
		}

		writeJSON(
			w,
			http.StatusOK,
			requestDomainRuleResponse{
				ProjectID:             projectID,
				AllowedRequestDomains: rule,
			},
		)
	}
}

func parseRequestDomainProjectID(r *http.Request) (int64, error) {
	value := strings.TrimSpace(r.PathValue("id"))
	if value == "" {
		return 0, errors.New("missing project id")
	}

	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid project id")
	}

	return id, nil
}

func bearerToken(header string) string {
	const prefix = "Bearer "

	if len(header) <= len(prefix) ||
		!strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}

	return strings.TrimSpace(header[len(prefix):])
}
