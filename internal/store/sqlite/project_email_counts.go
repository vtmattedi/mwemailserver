package sqlite

import (
	"context"

	"github.com/vtmattedi/mw-email-server/internal/domain"
)

func (s *Store) ListProjectEmailCounts(
	ctx context.Context,
) ([]domain.ProjectEmailCount, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`
		SELECT
			p.id,
			p.name,
			p.enabled,

			COUNT(e.project_id) AS email_count,

			SUM(CASE
				WHEN e.send_status = 'pending'
				THEN 1 ELSE 0
			END) AS send_pending,

			SUM(CASE
				WHEN e.send_status = 'sent'
				THEN 1 ELSE 0
			END) AS send_sent,

			SUM(CASE
				WHEN e.send_status = 'failed'
				THEN 1 ELSE 0
			END) AS send_failed,

			SUM(CASE
				WHEN e.delivery_status = 'sent'
				THEN 1 ELSE 0
			END) AS delivery_sent,

			SUM(CASE
				WHEN e.delivery_status = 'delivered'
				THEN 1 ELSE 0
			END) AS delivered,

			SUM(CASE
				WHEN e.delivery_status = 'delivery_delayed'
				THEN 1 ELSE 0
			END) AS delivery_delayed,

			SUM(CASE
				WHEN e.delivery_status = 'bounced'
				THEN 1 ELSE 0
			END) AS bounced,

			SUM(CASE
				WHEN e.delivery_status = 'failed'
				THEN 1 ELSE 0
			END) AS delivery_failed,

			SUM(CASE
				WHEN e.delivery_status = 'suppressed'
				THEN 1 ELSE 0
			END) AS suppressed,

			SUM(CASE
				WHEN e.delivery_status = 'complained'
				THEN 1 ELSE 0
			END) AS complained

		FROM projects AS p

		LEFT JOIN email_attempts AS e
			ON e.project_id = p.id

		GROUP BY
			p.id,
			p.name,
			p.enabled

		ORDER BY
			p.name COLLATE NOCASE ASC,
			p.id ASC
		`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := make([]domain.ProjectEmailCount, 0)

	for rows.Next() {
		var item domain.ProjectEmailCount
		var enabled int

		if err := rows.Scan(
			&item.ProjectID,
			&item.Name,
			&enabled,

			&item.EmailCount,

			&item.Send.Pending,
			&item.Send.Sent,
			&item.Send.Failed,

			&item.Delivery.Sent,
			&item.Delivery.Delivered,
			&item.Delivery.DeliveryDelayed,
			&item.Delivery.Bounced,
			&item.Delivery.Failed,
			&item.Delivery.Suppressed,
			&item.Delivery.Complained,
		); err != nil {
			return nil, err
		}

		item.Enabled = enabled != 0

		projects = append(projects, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return projects, nil
}