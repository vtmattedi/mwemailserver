// MW_ALLOWED_REQUEST_DOMAINS_V1
package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
)

func (s *Store) GetRequestDomainRuleForToken(
	ctx context.Context,
	rawToken string,
) (projectID int64, rule string, err error) {
	sum := sha256.Sum256([]byte(rawToken))
	hexHash := hex.EncodeToString(sum[:])

	err = s.db.QueryRowContext(
		ctx,
		`SELECT
			p.id,
			COALESCE(r.rule, '*')
		FROM projects AS p
		LEFT JOIN project_request_domain_rules AS r
			ON r.project_id = p.id
		WHERE p.enabled = 1
			AND (p.token_hash = ? OR p.token_hash = ?)
		LIMIT 1`,
		hexHash,
		sum[:],
	).Scan(&projectID, &rule)

	return projectID, rule, err
}

func (s *Store) GetProjectRequestDomainRule(
	ctx context.Context,
	projectID int64,
) (string, error) {
	var rule string

	err := s.db.QueryRowContext(
		ctx,
		`SELECT COALESCE(r.rule, '*')
		FROM projects AS p
		LEFT JOIN project_request_domain_rules AS r
			ON r.project_id = p.id
		WHERE p.id = ?`,
		projectID,
	).Scan(&rule)

	return rule, err
}

func (s *Store) SetProjectRequestDomainRule(
	ctx context.Context,
	projectID int64,
	rule string,
) error {
	var exists int

	if err := s.db.QueryRowContext(
		ctx,
		`SELECT 1 FROM projects WHERE id = ?`,
		projectID,
	).Scan(&exists); err != nil {
		return err
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO project_request_domain_rules (
			project_id,
			rule,
			created_at,
			updated_at
		) VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(project_id) DO UPDATE SET
			rule = excluded.rule,
			updated_at = CURRENT_TIMESTAMP`,
		projectID,
		rule,
	)

	return err
}
