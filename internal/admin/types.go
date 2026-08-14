package admin

import "time"

type Principal struct {
	Source          string
	LocalAdminID    *int64
	IdentitySubject *string
	Scopes          []string
	AuthTime        *time.Time
	AMR             []string
	Audience        string
}

type User struct {
	ID          int64      `json:"id"`
	Email       string     `json:"email"`
	Enabled     bool       `json:"enabled"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Session struct {
	ID            int64
	AdminUserID   int64
	TokenHash     string
	CSRFTokenHash string
	ExpiresAt     time.Time
	CreatedAt     time.Time
	LastUsedAt    time.Time
	User          User
}

type Project struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Enabled      bool      `json:"enabled"`
	MonthlyLimit *int64    `json:"monthly_limit"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type EmailAttempt struct {
	ID               int64      `json:"id"`
	ProjectID        int64      `json:"project_id"`
	ProjectName      string     `json:"project_name"`
	Provider         string     `json:"provider"`
	ProviderID       string     `json:"provider_id,omitempty"`
	Sender           string     `json:"sender"`
	Subject          string     `json:"subject"`
	RecipientCount   int        `json:"recipient_count"`
	Status           string     `json:"status"`
	ErrorMessage     string     `json:"error_message,omitempty"`
	DeliveryStatus   string     `json:"delivery_status,omitempty"`
	DeliveryStatusAt *time.Time `json:"delivery_status_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type EmailEvent struct {
	ID              int64     `json:"id"`
	Provider        string    `json:"provider"`
	ProviderEventID string    `json:"provider_event_id"`
	ProviderEmailID string    `json:"provider_email_id,omitempty"`
	EventType       string    `json:"event_type"`
	OccurredAt      time.Time `json:"occurred_at"`
	ReceivedAt      time.Time `json:"received_at"`
}

type EmailAttemptDetails struct {
	EmailAttempt
	Events []EmailEvent `json:"events"`
}

type UpdateProjectParams struct {
	ID                int64
	Name              *string
	Enabled           *bool
	MonthlyLimit      *int64
	ClearMonthlyLimit bool
}

type DeleteProjectParams struct {
	ID              int64
	ActorType       string
	IdentitySubject *string
	LocalAdminID    *int64
	RequestID       string
	OccurredAt      time.Time
}

type ListEmailAttemptsParams struct {
	Limit          int
	Offset         int
	ProjectID      *int64
	Status         string
	DeliveryStatus string
}
