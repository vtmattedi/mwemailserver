package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/vtmattedi/mw-email-server/internal/domain"
)

type ProjectEmailCountStore interface {
	ListProjectEmailCounts(
		ctx context.Context,
	) ([]domain.ProjectEmailCount, error)
}

type projectEmailCountsResponse struct {
	Projects []domain.ProjectEmailCount `json:"projects"`

	TotalEmails int64 `json:"total_emails"`

	Send     domain.EmailSendStatusCounts     `json:"send"`
	Delivery domain.EmailDeliveryStatusCounts `json:"delivery"`
}

func AdminProjectEmailCounts(
	store ProjectEmailCountStore,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projects, err := store.ListProjectEmailCounts(r.Context())
		if err != nil {
			slog.Error(
				"could not load project email counts",
				"error", err,
				"request_id", r.Header.Get("X-Request-ID"),
			)
			writeError(
				w,
				http.StatusInternalServerError,
				"could not load project email counts",
			)
			return
		}

		var response projectEmailCountsResponse

		response.Projects = projects

		for _, project := range projects {
			response.TotalEmails += project.EmailCount

			response.Send.Pending += project.Send.Pending
			response.Send.Sent += project.Send.Sent
			response.Send.Failed += project.Send.Failed

			response.Delivery.Sent += project.Delivery.Sent
			response.Delivery.Delivered += project.Delivery.Delivered
			response.Delivery.DeliveryDelayed +=
				project.Delivery.DeliveryDelayed

			response.Delivery.Bounced += project.Delivery.Bounced
			response.Delivery.Failed += project.Delivery.Failed
			response.Delivery.Suppressed += project.Delivery.Suppressed
			response.Delivery.Complained += project.Delivery.Complained
		}

		writeJSON(
			w,
			http.StatusOK,
			response,
		)
	}
}
