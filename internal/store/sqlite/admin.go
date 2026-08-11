package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/vtmattedi/mw-email-server/internal/admin"
)

func (s *Store) CreateAdminUser(
	ctx context.Context,
	email string,
	passwordHash string,
) (admin.User, error) {
	result, err := s.db.ExecContext(
		ctx,
		`INSERT INTO admin_users (
			email,
			password_hash
		) VALUES (?, ?)`,
		email,
		passwordHash,
	)
	if err != nil {
		return admin.User{}, fmt.Errorf("insert admin user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return admin.User{}, fmt.Errorf("get admin user id: %w", err)
	}

	return s.getAdminUserByID(ctx, id)
}

func (s *Store) GetAdminUserByEmail(
	ctx context.Context,
	email string,
) (admin.User, string, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT
			id,
			email,
			password_hash,
			enabled,
			last_login_at,
			created_at,
			updated_at
		 FROM admin_users
		 WHERE email = ? COLLATE NOCASE
		 LIMIT 1`,
		email,
	)

	return scanAdminUserWithPassword(row)
}

func (s *Store) getAdminUserByID(
	ctx context.Context,
	id int64,
) (admin.User, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT
			id,
			email,
			enabled,
			last_login_at,
			created_at,
			updated_at
		 FROM admin_users
		 WHERE id = ?
		 LIMIT 1`,
		id,
	)

	return scanAdminUser(row)
}

func (s *Store) CreateAdminSession(
	ctx context.Context,
	adminUserID int64,
	tokenHash string,
	csrfTokenHash string,
	expiresAt time.Time,
) (admin.Session, error) {
	result, err := s.db.ExecContext(
		ctx,
		`INSERT INTO admin_sessions (
			admin_user_id,
			token_hash,
			csrf_token_hash,
			expires_at
		) VALUES (?, ?, ?, ?)`,
		adminUserID,
		tokenHash,
		csrfTokenHash,
		expiresAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return admin.Session{}, fmt.Errorf("insert admin session: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return admin.Session{}, fmt.Errorf("get admin session id: %w", err)
	}

	return s.getAdminSessionByID(ctx, id)
}

func (s *Store) GetAdminSessionByTokenHash(
	ctx context.Context,
	tokenHash string,
) (admin.Session, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT
			s.id,
			s.admin_user_id,
			s.token_hash,
			s.csrf_token_hash,
			s.expires_at,
			s.created_at,
			s.last_used_at,
			u.id,
			u.email,
			u.enabled,
			u.last_login_at,
			u.created_at,
			u.updated_at
		 FROM admin_sessions s
		 JOIN admin_users u
		   ON u.id = s.admin_user_id
		 WHERE s.token_hash = ?
		 LIMIT 1`,
		tokenHash,
	)

	return scanAdminSession(row)
}

func (s *Store) getAdminSessionByID(
	ctx context.Context,
	id int64,
) (admin.Session, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT
			s.id,
			s.admin_user_id,
			s.token_hash,
			s.csrf_token_hash,
			s.expires_at,
			s.created_at,
			s.last_used_at,
			u.id,
			u.email,
			u.enabled,
			u.last_login_at,
			u.created_at,
			u.updated_at
		 FROM admin_sessions s
		 JOIN admin_users u
		   ON u.id = s.admin_user_id
		 WHERE s.id = ?
		 LIMIT 1`,
		id,
	)

	return scanAdminSession(row)
}

func (s *Store) UpdateAdminSessionCSRF(
	ctx context.Context,
	sessionID int64,
	csrfTokenHash string,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE admin_sessions
		 SET csrf_token_hash = ?,
		     last_used_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		csrfTokenHash,
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("update admin CSRF token: %w", err)
	}

	return nil
}

func (s *Store) TouchAdminSession(
	ctx context.Context,
	sessionID int64,
	at time.Time,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE admin_sessions
		 SET last_used_at = ?
		 WHERE id = ?`,
		at.UTC().Format(time.RFC3339Nano),
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("touch admin session: %w", err)
	}

	return nil
}

func (s *Store) DeleteAdminSession(
	ctx context.Context,
	sessionID int64,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`DELETE FROM admin_sessions WHERE id = ?`,
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("delete admin session: %w", err)
	}

	return nil
}

func (s *Store) DeleteExpiredAdminSessions(
	ctx context.Context,
	now time.Time,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`DELETE FROM admin_sessions
		 WHERE expires_at <= ?`,
		now.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("delete expired admin sessions: %w", err)
	}

	return nil
}

func (s *Store) UpdateAdminLastLogin(
	ctx context.Context,
	adminUserID int64,
	at time.Time,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE admin_users
		 SET last_login_at = ?,
		     updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		at.UTC().Format(time.RFC3339Nano),
		adminUserID,
	)
	if err != nil {
		return fmt.Errorf("update admin last login: %w", err)
	}

	return nil
}

func (s *Store) ListAdminProjects(
	ctx context.Context,
) ([]admin.Project, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT
			id,
			name,
			enabled,
			monthly_limit,
			created_at,
			updated_at
		 FROM projects
		 ORDER BY name COLLATE NOCASE ASC, id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	projects := make([]admin.Project, 0)

	for rows.Next() {
		project, err := scanAdminProject(rows)
		if err != nil {
			return nil, err
		}

		projects = append(projects, project)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}

	return projects, nil
}

func (s *Store) CreateAdminProject(
	ctx context.Context,
	name string,
	tokenHash string,
	monthlyLimit *int64,
) (admin.Project, error) {
	result, err := s.db.ExecContext(
		ctx,
		`INSERT INTO projects (
			name,
			token_hash,
			monthly_limit
		) VALUES (?, ?, ?)`,
		name,
		tokenHash,
		nullableInt64Pointer(monthlyLimit),
	)
	if err != nil {
		return admin.Project{}, fmt.Errorf("insert project: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return admin.Project{}, fmt.Errorf("get project id: %w", err)
	}

	return s.getAdminProjectByID(ctx, id)
}

func (s *Store) UpdateAdminProject(
	ctx context.Context,
	params admin.UpdateProjectParams,
) (admin.Project, error) {
	if params.ID <= 0 {
		return admin.Project{}, fmt.Errorf("invalid project id")
	}

	set := make([]string, 0, 4)
	args := make([]any, 0, 5)

	if params.Name != nil {
		set = append(set, "name = ?")
		args = append(args, *params.Name)
	}

	if params.Enabled != nil {
		set = append(set, "enabled = ?")
		if *params.Enabled {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}

	if params.ClearMonthlyLimit {
		set = append(set, "monthly_limit = NULL")
	} else if params.MonthlyLimit != nil {
		set = append(set, "monthly_limit = ?")
		args = append(args, *params.MonthlyLimit)
	}

	if len(set) == 0 {
		return s.getAdminProjectByID(ctx, params.ID)
	}

	set = append(set, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, params.ID)

	query := `UPDATE projects
		SET ` + strings.Join(set, ", ") + `
		WHERE id = ?`

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return admin.Project{}, fmt.Errorf("update project: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return admin.Project{}, fmt.Errorf("read project update result: %w", err)
	}
	if rows == 0 {
		return admin.Project{}, sql.ErrNoRows
	}

	return s.getAdminProjectByID(ctx, params.ID)
}

func (s *Store) RotateAdminProjectToken(
	ctx context.Context,
	projectID int64,
	tokenHash string,
) error {
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE projects
		 SET token_hash = ?,
		     updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		tokenHash,
		projectID,
	)
	if err != nil {
		return fmt.Errorf("rotate project token: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read project token update result: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s *Store) ListAdminEmailAttempts(
	ctx context.Context,
	params admin.ListEmailAttemptsParams,
) ([]admin.EmailAttempt, error) {
	query := `SELECT
			e.id,
			e.project_id,
			p.name,
			e.provider,
			COALESCE(e.provider_id, ''),
			e.sender,
			e.subject,
			e.recipient_count,
			e.status,
			COALESCE(e.error_message, ''),
			COALESCE(e.delivery_status, ''),
			e.delivery_status_at,
			e.created_at,
			e.updated_at
		FROM email_attempts e
		JOIN projects p ON p.id = e.project_id
		WHERE 1 = 1`

	args := make([]any, 0, 7)

	if params.ProjectID != nil {
		query += " AND e.project_id = ?"
		args = append(args, *params.ProjectID)
	}

	if params.Status != "" {
		query += " AND e.status = ?"
		args = append(args, params.Status)
	}

	if params.DeliveryStatus != "" {
		query += " AND e.delivery_status = ?"
		args = append(args, params.DeliveryStatus)
	}

	query += " ORDER BY e.id DESC LIMIT ? OFFSET ?"
	args = append(args, params.Limit, params.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list email attempts: %w", err)
	}
	defer rows.Close()

	attempts := make([]admin.EmailAttempt, 0)

	for rows.Next() {
		attempt, err := scanAdminEmailAttempt(rows)
		if err != nil {
			return nil, err
		}

		attempts = append(attempts, attempt)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate email attempts: %w", err)
	}

	return attempts, nil
}

func (s *Store) GetAdminEmailAttempt(
	ctx context.Context,
	id int64,
) (admin.EmailAttemptDetails, error) {
	attempt, err := scanAdminEmailAttempt(
		s.db.QueryRowContext(
			ctx,
			`SELECT
				e.id,
				e.project_id,
				p.name,
				e.provider,
				COALESCE(e.provider_id, ''),
				e.sender,
				e.subject,
				e.recipient_count,
				e.status,
				COALESCE(e.error_message, ''),
				COALESCE(e.delivery_status, ''),
				e.delivery_status_at,
				e.created_at,
				e.updated_at
			 FROM email_attempts e
			 JOIN projects p ON p.id = e.project_id
			 WHERE e.id = ?
			 LIMIT 1`,
			id,
		),
	)
	if err != nil {
		return admin.EmailAttemptDetails{}, err
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT
			id,
			provider,
			provider_event_id,
			COALESCE(provider_email_id, ''),
			event_type,
			occurred_at,
			received_at
		 FROM email_events
		 WHERE email_attempt_id = ?
		 ORDER BY occurred_at ASC, id ASC`,
		id,
	)
	if err != nil {
		return admin.EmailAttemptDetails{}, fmt.Errorf("list email events: %w", err)
	}
	defer rows.Close()

	events := make([]admin.EmailEvent, 0)

	for rows.Next() {
		var event admin.EmailEvent
		var occurredAt string
		var receivedAt string

		if err := rows.Scan(
			&event.ID,
			&event.Provider,
			&event.ProviderEventID,
			&event.ProviderEmailID,
			&event.EventType,
			&occurredAt,
			&receivedAt,
		); err != nil {
			return admin.EmailAttemptDetails{}, fmt.Errorf("scan email event: %w", err)
		}

		event.OccurredAt, err = parseSQLiteTime(occurredAt)
		if err != nil {
			return admin.EmailAttemptDetails{}, fmt.Errorf("parse event occurred_at: %w", err)
		}

		event.ReceivedAt, err = parseSQLiteTime(receivedAt)
		if err != nil {
			return admin.EmailAttemptDetails{}, fmt.Errorf("parse event received_at: %w", err)
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return admin.EmailAttemptDetails{}, fmt.Errorf("iterate email events: %w", err)
	}

	return admin.EmailAttemptDetails{
		EmailAttempt: attempt,
		Events:       events,
	}, nil
}

func (s *Store) getAdminProjectByID(
	ctx context.Context,
	id int64,
) (admin.Project, error) {
	return scanAdminProject(
		s.db.QueryRowContext(
			ctx,
			`SELECT
				id,
				name,
				enabled,
				monthly_limit,
				created_at,
				updated_at
			 FROM projects
			 WHERE id = ?
			 LIMIT 1`,
			id,
		),
	)
}

func scanAdminUser(row rowScanner) (admin.User, error) {
	var user admin.User
	var enabled int64
	var lastLogin sql.NullString
	var createdAt string
	var updatedAt string

	if err := row.Scan(
		&user.ID,
		&user.Email,
		&enabled,
		&lastLogin,
		&createdAt,
		&updatedAt,
	); err != nil {
		return admin.User{}, err
	}

	user.Enabled = enabled != 0

	var err error
	if lastLogin.Valid {
		value, parseErr := parseSQLiteTime(lastLogin.String)
		if parseErr != nil {
			return admin.User{}, fmt.Errorf("parse admin last_login_at: %w", parseErr)
		}
		user.LastLoginAt = &value
	}

	user.CreatedAt, err = parseSQLiteTime(createdAt)
	if err != nil {
		return admin.User{}, fmt.Errorf("parse admin created_at: %w", err)
	}

	user.UpdatedAt, err = parseSQLiteTime(updatedAt)
	if err != nil {
		return admin.User{}, fmt.Errorf("parse admin updated_at: %w", err)
	}

	return user, nil
}

func scanAdminUserWithPassword(
	row rowScanner,
) (admin.User, string, error) {
	var user admin.User
	var passwordHash string
	var enabled int64
	var lastLogin sql.NullString
	var createdAt string
	var updatedAt string

	if err := row.Scan(
		&user.ID,
		&user.Email,
		&passwordHash,
		&enabled,
		&lastLogin,
		&createdAt,
		&updatedAt,
	); err != nil {
		return admin.User{}, "", err
	}

	user.Enabled = enabled != 0

	var err error
	if lastLogin.Valid {
		value, parseErr := parseSQLiteTime(lastLogin.String)
		if parseErr != nil {
			return admin.User{}, "", fmt.Errorf("parse admin last_login_at: %w", parseErr)
		}
		user.LastLoginAt = &value
	}

	user.CreatedAt, err = parseSQLiteTime(createdAt)
	if err != nil {
		return admin.User{}, "", fmt.Errorf("parse admin created_at: %w", err)
	}

	user.UpdatedAt, err = parseSQLiteTime(updatedAt)
	if err != nil {
		return admin.User{}, "", fmt.Errorf("parse admin updated_at: %w", err)
	}

	return user, passwordHash, nil
}

func scanAdminSession(row rowScanner) (admin.Session, error) {
	var session admin.Session
	var expiresAt string
	var createdAt string
	var lastUsedAt string

	var userEnabled int64
	var userLastLogin sql.NullString
	var userCreatedAt string
	var userUpdatedAt string

	if err := row.Scan(
		&session.ID,
		&session.AdminUserID,
		&session.TokenHash,
		&session.CSRFTokenHash,
		&expiresAt,
		&createdAt,
		&lastUsedAt,
		&session.User.ID,
		&session.User.Email,
		&userEnabled,
		&userLastLogin,
		&userCreatedAt,
		&userUpdatedAt,
	); err != nil {
		return admin.Session{}, err
	}

	session.User.Enabled = userEnabled != 0

	var err error

	session.ExpiresAt, err = parseSQLiteTime(expiresAt)
	if err != nil {
		return admin.Session{}, fmt.Errorf("parse admin session expires_at: %w", err)
	}

	session.CreatedAt, err = parseSQLiteTime(createdAt)
	if err != nil {
		return admin.Session{}, fmt.Errorf("parse admin session created_at: %w", err)
	}

	session.LastUsedAt, err = parseSQLiteTime(lastUsedAt)
	if err != nil {
		return admin.Session{}, fmt.Errorf("parse admin session last_used_at: %w", err)
	}

	if userLastLogin.Valid {
		value, parseErr := parseSQLiteTime(userLastLogin.String)
		if parseErr != nil {
			return admin.Session{}, fmt.Errorf("parse admin last_login_at: %w", parseErr)
		}
		session.User.LastLoginAt = &value
	}

	session.User.CreatedAt, err = parseSQLiteTime(userCreatedAt)
	if err != nil {
		return admin.Session{}, fmt.Errorf("parse admin created_at: %w", err)
	}

	session.User.UpdatedAt, err = parseSQLiteTime(userUpdatedAt)
	if err != nil {
		return admin.Session{}, fmt.Errorf("parse admin updated_at: %w", err)
	}

	return session, nil
}

func scanAdminProject(row rowScanner) (admin.Project, error) {
	var project admin.Project
	var enabled int64
	var monthlyLimit sql.NullInt64
	var createdAt string
	var updatedAt string

	if err := row.Scan(
		&project.ID,
		&project.Name,
		&enabled,
		&monthlyLimit,
		&createdAt,
		&updatedAt,
	); err != nil {
		return admin.Project{}, err
	}

	project.Enabled = enabled != 0

	if monthlyLimit.Valid {
		value := monthlyLimit.Int64
		project.MonthlyLimit = &value
	}

	var err error
	project.CreatedAt, err = parseSQLiteTime(createdAt)
	if err != nil {
		return admin.Project{}, fmt.Errorf("parse project created_at: %w", err)
	}

	project.UpdatedAt, err = parseSQLiteTime(updatedAt)
	if err != nil {
		return admin.Project{}, fmt.Errorf("parse project updated_at: %w", err)
	}

	return project, nil
}

func scanAdminEmailAttempt(row rowScanner) (admin.EmailAttempt, error) {
	var attempt admin.EmailAttempt
	var deliveryStatusAt sql.NullString
	var createdAt string
	var updatedAt string

	if err := row.Scan(
		&attempt.ID,
		&attempt.ProjectID,
		&attempt.ProjectName,
		&attempt.Provider,
		&attempt.ProviderID,
		&attempt.Sender,
		&attempt.Subject,
		&attempt.RecipientCount,
		&attempt.Status,
		&attempt.ErrorMessage,
		&attempt.DeliveryStatus,
		&deliveryStatusAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return admin.EmailAttempt{}, err
	}

	var err error

	if deliveryStatusAt.Valid {
		value, parseErr := parseSQLiteTime(deliveryStatusAt.String)
		if parseErr != nil {
			return admin.EmailAttempt{}, fmt.Errorf(
				"parse email delivery_status_at: %w",
				parseErr,
			)
		}
		attempt.DeliveryStatusAt = &value
	}

	attempt.CreatedAt, err = parseSQLiteTime(createdAt)
	if err != nil {
		return admin.EmailAttempt{}, fmt.Errorf("parse email created_at: %w", err)
	}

	attempt.UpdatedAt, err = parseSQLiteTime(updatedAt)
	if err != nil {
		return admin.EmailAttempt{}, fmt.Errorf("parse email updated_at: %w", err)
	}

	return attempt, nil
}

func nullableInt64Pointer(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}
