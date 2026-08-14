package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vtmattedi/mw-email-server/internal/admin"
)

const maxAdminRequestBody = 256 * 1024

type adminLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type adminLoginResponse struct {
	User      admin.User `json:"user"`
	CSRFToken string     `json:"csrf_token"`
}

type adminCSRFResponse struct {
	CSRFToken string `json:"csrf_token"`
}

type createProjectRequest struct {
	Name         string `json:"name"`
	MonthlyLimit *int64 `json:"monthly_limit"`
}

type createProjectResponse struct {
	Project admin.Project `json:"project"`
	APIKey  string        `json:"api_key"`
}

type updateProjectRequest struct {
	Name              *string `json:"name"`
	Enabled           *bool   `json:"enabled"`
	MonthlyLimit      *int64  `json:"monthly_limit"`
	ClearMonthlyLimit bool    `json:"clear_monthly_limit"`
}

type rotateProjectKeyResponse struct {
	APIKey string `json:"api_key"`
}

func AdminLogin(
	service *admin.Service,
	secureCookie bool,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBody)

		var req adminLoginRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		if err := decoder.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		result, err := service.Login(
			r.Context(),
			req.Email,
			req.Password,
		)
		if err != nil {
			switch {
			case errors.Is(err, admin.ErrInvalidCredentials),
				errors.Is(err, admin.ErrAdminDisabled):
				writeError(w, http.StatusUnauthorized, "invalid credentials")
			default:
				writeError(w, http.StatusInternalServerError, "login failed")
			}
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     admin.SessionCookieName,
			Value:    result.SessionToken,
			Path:     "/admin/api",
			HttpOnly: true,
			Secure:   secureCookie,
			SameSite: http.SameSiteStrictMode,
			Expires:  result.ExpiresAt,
			MaxAge:   int(timeUntilSeconds(result.ExpiresAt)),
		})

		writeJSON(w, http.StatusOK, adminLoginResponse{
			User:      result.User,
			CSRFToken: result.CSRFToken,
		})
	}
}

func AdminMe(w http.ResponseWriter, r *http.Request) {
	session, ok := admin.SessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": session.User,
	})
}

func AdminRotateCSRF(service *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := admin.SessionFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		token, err := service.RotateCSRF(r.Context(), session)
		if err != nil {
			writeError(
				w,
				http.StatusInternalServerError,
				"failed to rotate CSRF token",
			)
			return
		}

		writeJSON(w, http.StatusOK, adminCSRFResponse{
			CSRFToken: token,
		})
	}
}

func AdminLogout(
	service *admin.Service,
	secureCookie bool,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := admin.SessionFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		if err := service.Logout(r.Context(), session.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "logout failed")
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     admin.SessionCookieName,
			Value:    "",
			Path:     "/admin/api",
			HttpOnly: true,
			Secure:   secureCookie,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   -1,
		})

		w.WriteHeader(http.StatusNoContent)
	}
}

func AdminHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func AdminListProjects(service *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projects, err := service.ListProjects(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list projects")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"projects": projects,
		})
	}
}

func AdminCreateProject(service *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBody)

		var req createProjectRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		if err := decoder.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		result, err := service.CreateProject(
			r.Context(),
			req.Name,
			req.MonthlyLimit,
		)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, createProjectResponse{
			Project: result.Project,
			APIKey:  result.APIKey,
		})
	}
}

func AdminUpdateProject(service *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID, err := positivePathID(r, "id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid project id")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBody)

		var req updateProjectRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		if err := decoder.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.ClearMonthlyLimit && req.MonthlyLimit != nil {
			writeError(
				w,
				http.StatusBadRequest,
				"monthly_limit and clear_monthly_limit cannot be used together",
			)
			return
		}

		project, err := service.UpdateProject(
			r.Context(),
			admin.UpdateProjectParams{
				ID:                projectID,
				Name:              req.Name,
				Enabled:           req.Enabled,
				MonthlyLimit:      req.MonthlyLimit,
				ClearMonthlyLimit: req.ClearMonthlyLimit,
			},
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}

			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"project": project,
		})
	}
}

func AdminRotateProjectKey(service *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID, err := positivePathID(r, "id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid project id")
			return
		}

		apiKey, err := service.RotateProjectKey(r.Context(), projectID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}

			writeError(
				w,
				http.StatusInternalServerError,
				"failed to rotate project API key",
			)
			return
		}

		writeJSON(w, http.StatusOK, rotateProjectKeyResponse{
			APIKey: apiKey,
		})
	}
}

func AdminDeleteProject(service *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID, err := positivePathID(r, "id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid project id")
			return
		}

		_, err = service.DeleteProject(
			r.Context(),
			projectID,
			r.Header.Get("X-Request-ID"),
		)
		if err != nil {
			switch {
			case errors.Is(err, sql.ErrNoRows):
				writeError(w, http.StatusNotFound, "project not found")
			case errors.Is(err, admin.ErrProjectHasDependencies):
				writeErrorCode(
					w,
					http.StatusConflict,
					"project_has_dependencies",
					"project has retained email history",
				)
			default:
				slog.Error(
					"failed to delete project",
					"error", err,
					"project_id", projectID,
					"request_id", r.Header.Get("X-Request-ID"),
				)
				writeError(w, http.StatusInternalServerError, "failed to delete project")
			}
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func AdminListEmails(service *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := admin.ListEmailAttemptsParams{
			Limit:          parseQueryInt(r, "limit", 50),
			Offset:         parseQueryInt(r, "offset", 0),
			Status:         strings.TrimSpace(r.URL.Query().Get("status")),
			DeliveryStatus: strings.TrimSpace(r.URL.Query().Get("delivery_status")),
		}

		if raw := strings.TrimSpace(r.URL.Query().Get("project_id")); raw != "" {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || value <= 0 {
				writeError(w, http.StatusBadRequest, "invalid project_id")
				return
			}
			params.ProjectID = &value
		}

		attempts, err := service.ListEmailAttempts(r.Context(), params)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list emails")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"emails": attempts,
			"limit":  params.Limit,
			"offset": params.Offset,
		})
	}
}

func AdminGetEmail(service *admin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := positivePathID(r, "id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid email id")
			return
		}

		attempt, err := service.GetEmailAttempt(r.Context(), id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "email not found")
				return
			}

			writeError(w, http.StatusInternalServerError, "failed to load email")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"email": attempt,
		})
	}
}

func positivePathID(r *http.Request, name string) (int64, error) {
	value, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || value <= 0 {
		return 0, errors.New("invalid id")
	}

	return value, nil
}

func parseQueryInt(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	return value
}

func timeUntilSeconds(expiresAt time.Time) int64 {
	seconds := int64(time.Until(expiresAt).Seconds())
	if seconds < 1 {
		return 1
	}
	return seconds
}
