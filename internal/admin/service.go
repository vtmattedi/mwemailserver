package admin

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	projectauth "github.com/vtmattedi/mw-email-server/internal/auth"
)

const DefaultSessionTTL = 7 * 24 * time.Hour

var (
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrAdminDisabled          = errors.New("admin account is disabled")
	ErrInvalidSession         = errors.New("invalid admin session")
	ErrExpiredSession         = errors.New("admin session has expired")
	ErrInvalidCSRF            = errors.New("invalid CSRF token")
	ErrProjectHasDependencies = errors.New("project has retained email history")
	ErrAdminPrincipalMissing  = errors.New("trusted admin principal is required")
)

type Service struct {
	store      Store
	sessionTTL time.Duration
}

type LoginResult struct {
	User         User
	SessionToken string
	CSRFToken    string
	ExpiresAt    time.Time
}

type ProjectCredential struct {
	Project Project
	APIKey  string
}

func NewService(store Store, sessionTTL time.Duration) *Service {
	if sessionTTL <= 0 {
		sessionTTL = DefaultSessionTTL
	}

	return &Service{
		store:      store,
		sessionTTL: sessionTTL,
	}
}

func (s *Service) CreateAdmin(
	ctx context.Context,
	email string,
	password string,
) (User, error) {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return User{}, err
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}

	return s.store.CreateAdminUser(ctx, normalized, passwordHash)
}

func (s *Service) Login(
	ctx context.Context,
	email string,
	password string,
) (LoginResult, error) {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	user, passwordHash, err := s.store.GetAdminUserByEmail(ctx, normalized)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, err
	}

	valid, err := VerifyPassword(password, passwordHash)
	if err != nil {
		return LoginResult{}, fmt.Errorf("verify password: %w", err)
	}
	if !valid {
		return LoginResult{}, ErrInvalidCredentials
	}

	if !user.Enabled {
		return LoginResult{}, ErrAdminDisabled
	}

	sessionToken, err := generateRandomToken()
	if err != nil {
		return LoginResult{}, fmt.Errorf("generate session token: %w", err)
	}

	csrfToken, err := generateRandomToken()
	if err != nil {
		return LoginResult{}, fmt.Errorf("generate CSRF token: %w", err)
	}

	now := time.Now().UTC()
	expiresAt := now.Add(s.sessionTTL)

	if err := s.store.DeleteExpiredAdminSessions(ctx, now); err != nil {
		return LoginResult{}, fmt.Errorf("delete expired admin sessions: %w", err)
	}

	_, err = s.store.CreateAdminSession(
		ctx,
		user.ID,
		hashToken(sessionToken),
		hashToken(csrfToken),
		expiresAt,
	)
	if err != nil {
		return LoginResult{}, fmt.Errorf("create admin session: %w", err)
	}

	if err := s.store.UpdateAdminLastLogin(ctx, user.ID, now); err != nil {
		return LoginResult{}, fmt.Errorf("update admin last login: %w", err)
	}

	user.LastLoginAt = &now

	return LoginResult{
		User:         user,
		SessionToken: sessionToken,
		CSRFToken:    csrfToken,
		ExpiresAt:    expiresAt,
	}, nil
}

func (s *Service) Authenticate(
	ctx context.Context,
	rawSessionToken string,
) (Session, error) {
	if strings.TrimSpace(rawSessionToken) == "" {
		return Session{}, ErrInvalidSession
	}

	session, err := s.store.GetAdminSessionByTokenHash(
		ctx,
		hashToken(rawSessionToken),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrInvalidSession
		}
		return Session{}, err
	}

	now := time.Now().UTC()

	if !session.User.Enabled {
		_ = s.store.DeleteAdminSession(ctx, session.ID)
		return Session{}, ErrAdminDisabled
	}

	if !session.ExpiresAt.After(now) {
		_ = s.store.DeleteAdminSession(ctx, session.ID)
		return Session{}, ErrExpiredSession
	}

	if err := s.store.TouchAdminSession(ctx, session.ID, now); err != nil {
		return Session{}, fmt.Errorf("touch admin session: %w", err)
	}
	session.LastUsedAt = now

	return session, nil
}

func (s *Service) RotateCSRF(
	ctx context.Context,
	session Session,
) (string, error) {
	token, err := generateRandomToken()
	if err != nil {
		return "", fmt.Errorf("generate CSRF token: %w", err)
	}

	if err := s.store.UpdateAdminSessionCSRF(
		ctx,
		session.ID,
		hashToken(token),
	); err != nil {
		return "", fmt.Errorf("update CSRF token: %w", err)
	}

	return token, nil
}

func (s *Service) VerifyCSRF(session Session, rawToken string) bool {
	if rawToken == "" || session.CSRFTokenHash == "" {
		return false
	}

	actual := hashToken(rawToken)
	return subtle.ConstantTimeCompare(
		[]byte(actual),
		[]byte(session.CSRFTokenHash),
	) == 1
}

func (s *Service) Logout(ctx context.Context, sessionID int64) error {
	return s.store.DeleteAdminSession(ctx, sessionID)
}

func (s *Service) ListProjects(ctx context.Context) ([]Project, error) {
	return s.store.ListAdminProjects(ctx)
}

func (s *Service) CreateProject(
	ctx context.Context,
	name string,
	monthlyLimit *int64,
) (ProjectCredential, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ProjectCredential{}, fmt.Errorf("project name is required")
	}

	if monthlyLimit != nil && *monthlyLimit < 0 {
		return ProjectCredential{}, fmt.Errorf("monthly_limit cannot be negative")
	}

	apiKey, err := projectauth.GenerateToken()
	if err != nil {
		return ProjectCredential{}, fmt.Errorf("generate project API key: %w", err)
	}

	project, err := s.store.CreateAdminProject(
		ctx,
		name,
		projectauth.HashToken(apiKey),
		monthlyLimit,
	)
	if err != nil {
		return ProjectCredential{}, err
	}

	return ProjectCredential{
		Project: project,
		APIKey:  apiKey,
	}, nil
}

func (s *Service) UpdateProject(
	ctx context.Context,
	params UpdateProjectParams,
) (Project, error) {
	if params.Name != nil {
		value := strings.TrimSpace(*params.Name)
		if value == "" {
			return Project{}, fmt.Errorf("project name cannot be empty")
		}
		params.Name = &value
	}

	if params.MonthlyLimit != nil && *params.MonthlyLimit < 0 {
		return Project{}, fmt.Errorf("monthly_limit cannot be negative")
	}

	return s.store.UpdateAdminProject(ctx, params)
}

func (s *Service) RotateProjectKey(
	ctx context.Context,
	projectID int64,
) (string, error) {
	apiKey, err := projectauth.GenerateToken()
	if err != nil {
		return "", fmt.Errorf("generate project API key: %w", err)
	}

	if err := s.store.RotateAdminProjectToken(
		ctx,
		projectID,
		projectauth.HashToken(apiKey),
	); err != nil {
		return "", err
	}

	return apiKey, nil
}

func (s *Service) DeleteProject(
	ctx context.Context,
	projectID int64,
	requestID string,
) (Project, error) {
	if projectID <= 0 {
		return Project{}, fmt.Errorf("invalid project id")
	}

	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		return Project{}, ErrAdminPrincipalMissing
	}

	params := DeleteProjectParams{
		ID:         projectID,
		ActorType:  principal.Source,
		RequestID:  strings.TrimSpace(requestID),
		OccurredAt: time.Now().UTC(),
	}
	switch principal.Source {
	case "federated":
		if principal.IdentitySubject == nil || strings.TrimSpace(*principal.IdentitySubject) == "" {
			return Project{}, ErrAdminPrincipalMissing
		}
		subject := strings.TrimSpace(*principal.IdentitySubject)
		params.IdentitySubject = &subject
	case "local":
		if principal.LocalAdminID == nil || *principal.LocalAdminID <= 0 {
			return Project{}, ErrAdminPrincipalMissing
		}
		params.LocalAdminID = principal.LocalAdminID
	default:
		return Project{}, ErrAdminPrincipalMissing
	}

	return s.store.DeleteAdminProject(ctx, params)
}

func (s *Service) ListEmailAttempts(
	ctx context.Context,
	params ListEmailAttemptsParams,
) ([]EmailAttempt, error) {
	if params.Limit <= 0 {
		params.Limit = 50
	}
	if params.Limit > 200 {
		params.Limit = 200
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	return s.store.ListAdminEmailAttempts(ctx, params)
}

func (s *Service) GetEmailAttempt(
	ctx context.Context,
	id int64,
) (EmailAttemptDetails, error) {
	return s.store.GetAdminEmailAttempt(ctx, id)
}
