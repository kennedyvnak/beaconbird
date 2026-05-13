package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kennedyvnak/beaconbird/internal/domain"
	"github.com/kennedyvnak/beaconbird/internal/repository/sqlc"
)

type contextKey int

const keyAPIKey contextKey = iota

type apiKeyQuerier interface {
	GetAPIKeyByHash(ctx context.Context, keyHash string) (sqlc.ApiKey, error)
	TouchAPIKeyLastUsed(ctx context.Context, arg sqlc.TouchAPIKeyLastUsedParams) error
}

type Authenticator struct {
	q apiKeyQuerier
}

func NewAuthenticator(q apiKeyQuerier) *Authenticator {
	return &Authenticator{q: q}
}

func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(token, "bb_live_") && !strings.HasPrefix(token, "bb_test_") {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		hash := hashKey(token)
		row, err := a.q.GetAPIKeyByHash(r.Context(), hash)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		go func() {
			_ = a.q.TouchAPIKeyLastUsed(context.Background(), sqlc.TouchAPIKeyLastUsedParams{
				ID:         row.ID,
				LastUsedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			})
		}()

		key := &domain.APIKey{
			ID:        row.ID,
			Name:      row.Name,
			KeyHash:   row.KeyHash,
			Mode:      domain.APIKeyMode(row.Mode),
			IsRevoked: row.IsRevoked,
		}

		ctx := context.WithValue(r.Context(), keyAPIKey, key)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func APIKeyFromContext(ctx context.Context) *domain.APIKey {
	k, _ := ctx.Value(keyAPIKey).(*domain.APIKey)
	return k
}

func ContextWithAPIKey(ctx context.Context, key *domain.APIKey) context.Context {
	return context.WithValue(ctx, keyAPIKey, key)
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

func hashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
