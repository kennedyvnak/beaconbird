package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/kennedyvnak/beaconbird/internal/domain"
	"github.com/kennedyvnak/beaconbird/internal/repository/sqlc"
)

type APIKeyRepo struct {
	q *sqlc.Queries
}

func NewAPIKeyRepo(q *sqlc.Queries) *APIKeyRepo {
	return &APIKeyRepo{q: q}
}

func (r *APIKeyRepo) Create(ctx context.Context, k *domain.APIKey) (*domain.APIKey, error) {
	row, err := r.q.CreateAPIKey(ctx, sqlc.CreateAPIKeyParams{
		ID:      k.ID,
		Name:    k.Name,
		KeyHash: k.KeyHash,
		Mode:    string(k.Mode),
	})
	if err != nil {
		return nil, fmt.Errorf("apiKeyRepo.Create: %w", err)
	}
	return toAPIKey(row), nil
}

func (r *APIKeyRepo) GetByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	row, err := r.q.GetAPIKeyByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("apiKeyRepo.GetByHash: %w", err)
	}
	return toAPIKey(row), nil
}

func (r *APIKeyRepo) List(ctx context.Context) ([]*domain.APIKey, error) {
	rows, err := r.q.ListAPIKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("apiKeyRepo.List: %w", err)
	}
	result := make([]*domain.APIKey, len(rows))
	for i, row := range rows {
		result[i] = toAPIKey(row)
	}
	return result, nil
}

func (r *APIKeyRepo) Revoke(ctx context.Context, id string) (*domain.APIKey, error) {
	row, err := r.q.RevokeAPIKey(ctx, sqlc.RevokeAPIKeyParams{
		ID:        id,
		RevokedAt: toPgTime(time.Now().UTC()),
	})
	if err != nil {
		return nil, fmt.Errorf("apiKeyRepo.Revoke: %w", err)
	}
	return toAPIKey(row), nil
}

func (r *APIKeyRepo) TouchLastUsed(ctx context.Context, id string) error {
	err := r.q.TouchAPIKeyLastUsed(ctx, sqlc.TouchAPIKeyLastUsedParams{
		ID:         id,
		LastUsedAt: toPgTime(time.Now().UTC()),
	})
	if err != nil {
		return fmt.Errorf("apiKeyRepo.TouchLastUsed: %w", err)
	}
	return nil
}

func toAPIKey(row sqlc.ApiKey) *domain.APIKey {
	return &domain.APIKey{
		ID:         row.ID,
		Name:       row.Name,
		KeyHash:    row.KeyHash,
		Mode:       domain.APIKeyMode(row.Mode),
		IsRevoked:  row.IsRevoked,
		LastUsedAt: toTimePtr(row.LastUsedAt),
		CreatedAt:  toTime(row.CreatedAt),
		RevokedAt:  toTimePtr(row.RevokedAt),
	}
}
