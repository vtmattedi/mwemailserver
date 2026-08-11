package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vtmattedi/mw-email-server/internal/domain"
)

func (s *Store) GetProjectByTokenHash(
	ctx context.Context,
	hash string,
) (*domain.Project, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, name, token_hash, enabled, monthly_limit
		 FROM projects
		 WHERE token_hash = ?
		 LIMIT 1`,
		hash,
	)

	var project domain.Project
	var enabled int64
	var monthlyLimit sql.NullInt64

	if err := row.Scan(
		&project.ID,
		&project.Name,
		&project.TokenHash,
		&enabled,
		&monthlyLimit,
	); err != nil {
		return nil, err
	}

	project.Enabled = enabled != 0

	if monthlyLimit.Valid {
		value := monthlyLimit.Int64
		project.MonthlyLimit = &value
	}

	return &project, nil
}

func (s *Store) CreateProject(
	ctx context.Context,
	name string,
	tokenHash string,
) (int64, error) {
	result, err := s.db.ExecContext(
		ctx,
		`INSERT INTO projects (name, token_hash)
		 VALUES (?, ?)`,
		name,
		tokenHash,
	)
	if err != nil {
		return 0, fmt.Errorf("insert project: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get project id: %w", err)
	}

	return id, nil
}
