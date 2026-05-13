package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kennedyvnak/beaconbird/internal/api/dto"
	"github.com/kennedyvnak/beaconbird/internal/domain"
)

type apiKeyRepo interface {
	Create(ctx context.Context, key *domain.APIKey) (*domain.APIKey, error)
	List(ctx context.Context) ([]*domain.APIKey, error)
	Delete(ctx context.Context, id string) error
}

type APIKeyHandler struct {
	repo         apiKeyRepo
	keyGenerator func(mode string) (rawKey string, hash string, err error)
	idGenerator  func() string
}

func NewAPIKeyHandler(repo apiKeyRepo) *APIKeyHandler {
	return &APIKeyHandler{
		repo:         repo,
		keyGenerator: generateAPIKey,
		idGenerator:  newHandlerID,
	}
}

func (h *APIKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errors.New("name is required"))
		return
	}
	if req.Mode != string(domain.APIKeyModeLive) && req.Mode != string(domain.APIKeyModeTest) {
		writeError(w, http.StatusBadRequest, errors.New("mode must be live or test"))
		return
	}

	rawKey, hash, err := h.keyGenerator(req.Mode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	created, err := h.repo.Create(r.Context(), &domain.APIKey{
		ID:      h.idGenerator(),
		Name:    req.Name,
		KeyHash: hash,
		Mode:    domain.APIKeyMode(req.Mode),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, dto.APIKeyCreateResponse{
		ID:         created.ID,
		Name:       created.Name,
		Mode:       string(created.Mode),
		CreatedAt:  formatTime(created.CreatedAt),
		LastUsedAt: formatTimePtr(created.LastUsedAt),
		RawKey:     rawKey,
	})
}

func (h *APIKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.repo.List(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}

	resp := make([]dto.APIKeyResponse, 0, len(rows))
	for _, row := range rows {
		resp = append(resp, dto.APIKeyResponse{
			ID:         row.ID,
			Name:       row.Name,
			Mode:       string(row.Mode),
			CreatedAt:  formatTime(row.CreatedAt),
			LastUsedAt: formatTimePtr(row.LastUsedAt),
			IsRevoked:  row.IsRevoked,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *APIKeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func generateAPIKey(mode string) (string, string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate api key: %w", err)
	}
	rawKey := fmt.Sprintf("bb_%s_%s", mode, hex.EncodeToString(buf))
	sum := sha256.Sum256([]byte(rawKey))
	return rawKey, hex.EncodeToString(sum[:]), nil
}

func newHandlerID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
