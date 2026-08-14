package admin

import (
	"context"
	"time"
)

type Store interface {
	CreateAdminUser(
		ctx context.Context,
		email string,
		passwordHash string,
	) (User, error)

	GetAdminUserByEmail(
		ctx context.Context,
		email string,
	) (User, string, error)

	CreateAdminSession(
		ctx context.Context,
		adminUserID int64,
		tokenHash string,
		csrfTokenHash string,
		expiresAt time.Time,
	) (Session, error)

	GetAdminSessionByTokenHash(
		ctx context.Context,
		tokenHash string,
	) (Session, error)

	UpdateAdminSessionCSRF(
		ctx context.Context,
		sessionID int64,
		csrfTokenHash string,
	) error

	TouchAdminSession(
		ctx context.Context,
		sessionID int64,
		at time.Time,
	) error

	DeleteAdminSession(
		ctx context.Context,
		sessionID int64,
	) error

	DeleteExpiredAdminSessions(
		ctx context.Context,
		now time.Time,
	) error

	UpdateAdminLastLogin(
		ctx context.Context,
		adminUserID int64,
		at time.Time,
	) error

	ListAdminProjects(
		ctx context.Context,
	) ([]Project, error)

	CreateAdminProject(
		ctx context.Context,
		name string,
		tokenHash string,
		monthlyLimit *int64,
	) (Project, error)

	UpdateAdminProject(
		ctx context.Context,
		params UpdateProjectParams,
	) (Project, error)

	RotateAdminProjectToken(
		ctx context.Context,
		projectID int64,
		tokenHash string,
	) error

	DeleteAdminProject(
		ctx context.Context,
		params DeleteProjectParams,
	) (Project, error)

	ListAdminEmailAttempts(
		ctx context.Context,
		params ListEmailAttemptsParams,
	) ([]EmailAttempt, error)

	GetAdminEmailAttempt(
		ctx context.Context,
		id int64,
	) (EmailAttemptDetails, error)
}
