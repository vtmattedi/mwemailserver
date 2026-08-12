package domain

type EmailSendStatusCounts struct {
	Pending int64 `json:"pending"`
	Sent    int64 `json:"sent"`
	Failed  int64 `json:"failed"`
}

type EmailDeliveryStatusCounts struct {
	Sent            int64 `json:"sent"`
	Delivered       int64 `json:"delivered"`
	DeliveryDelayed int64 `json:"delivery_delayed"`
	Bounced         int64 `json:"bounced"`
	Failed          int64 `json:"failed"`
	Suppressed      int64 `json:"suppressed"`
	Complained      int64 `json:"complained"`
}

type ProjectEmailCount struct {
	ProjectID int64  `json:"project_id"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`

	EmailCount int64 `json:"email_count"`

	Send     EmailSendStatusCounts     `json:"send"`
	Delivery EmailDeliveryStatusCounts `json:"delivery"`
}