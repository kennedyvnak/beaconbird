package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kennedyvnak/beaconbird/config"
	"github.com/kennedyvnak/beaconbird/internal/api/handler"
	"github.com/kennedyvnak/beaconbird/internal/api/middleware"
	"github.com/kennedyvnak/beaconbird/internal/domain"
	"github.com/kennedyvnak/beaconbird/internal/repository"
	"github.com/kennedyvnak/beaconbird/internal/repository/sqlc"
	"github.com/kennedyvnak/beaconbird/internal/service"
)

func main() {
	if err := run(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	queries := sqlc.New(pool)
	auth := middleware.NewAuthenticator(queries)
	health := handler.NewHealthHandler()
	notificationRepo := repository.NewNotificationRepo(pool, queries)
	deliveryJobRepo := repository.NewDeliveryJobRepo(pool, queries)
	deliveryAttemptRepo := repository.NewDeliveryAttemptRepo(queries)
	apiKeyRepo := repository.NewAPIKeyRepo(pool, queries)
	notificationService := service.NewNotificationService(notificationRepo, deliveryJobRepo, repository.NewTxRunner(pool))
	notificationHandler := handler.NewNotificationHandler(
		notificationService,
		notificationReaderAdapter{repo: notificationRepo},
		deliveryJobRepo,
	)
	deliveryJobHandler := handler.NewDeliveryJobHandler(
		deliveryJobReaderAdapter{repo: deliveryJobRepo},
		deliveryAttemptRepo,
	)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeyRepo)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	r.Get("/v1/health", health.Get)

	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware)
		r.Route("/v1/notifications", func(r chi.Router) {
			r.Post("/", notificationHandler.Create)
			r.Post("/bulk", notificationHandler.BulkCreate)
			r.Get("/", notificationHandler.List)
			r.Get("/{id}", notificationHandler.Get)
			r.Patch("/{id}", notificationHandler.Patch)
			r.Post("/{id}/edit", notificationHandler.Edit)
		})
		r.Route("/v1/delivery-jobs", func(r chi.Router) {
			r.Get("/", deliveryJobHandler.List)
			r.Get("/{id}", deliveryJobHandler.Get)
		})
		r.Route("/v1/api-keys", func(r chi.Router) {
			r.Post("/", apiKeyHandler.Create)
			r.Get("/", apiKeyHandler.List)
			r.Delete("/{id}", apiKeyHandler.Delete)
		})
	})

	srv := &http.Server{
		Addr:         ":" + cfg.APIPort,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("api listening on :%s", cfg.APIPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("server: %w", err)
	case <-quit:
	}
	log.Println("shutting down...")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	return srv.Shutdown(shutCtx)
}

type notificationReaderAdapter struct {
	repo interface {
		GetByID(ctx context.Context, id string) (*domain.Notification, error)
		List(ctx context.Context, params repository.ListNotificationsParams) ([]*domain.Notification, error)
	}
}

func (a notificationReaderAdapter) GetByID(ctx context.Context, id string) (*domain.Notification, error) {
	notification, err := a.repo.GetByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, service.ErrNotFound
	}
	return notification, err
}

func (a notificationReaderAdapter) List(ctx context.Context, params handler.NotificationListParams) ([]*domain.Notification, error) {
	return a.repo.List(ctx, repository.ListNotificationsParams{
		Status:  params.Status,
		Tag:     params.Tag,
		Channel: params.Channel,
		From:    params.From,
		To:      params.To,
		Offset:  params.Offset,
		Limit:   params.Limit,
	})
}

type deliveryJobReaderAdapter struct {
	repo interface {
		GetByID(ctx context.Context, id string) (*domain.DeliveryJob, error)
		List(ctx context.Context, params repository.ListDeliveryJobsParams) ([]*domain.DeliveryJob, error)
	}
}

func (a deliveryJobReaderAdapter) GetByID(ctx context.Context, id string) (*domain.DeliveryJob, error) {
	job, err := a.repo.GetByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, service.ErrNotFound
	}
	return job, err
}

func (a deliveryJobReaderAdapter) List(ctx context.Context, params handler.DeliveryJobListParams) ([]*domain.DeliveryJob, error) {
	return a.repo.List(ctx, repository.ListDeliveryJobsParams{
		Status:         params.Status,
		Channel:        params.Channel,
		NotificationID: params.NotificationID,
		From:           params.From,
		To:             params.To,
		Offset:         params.Offset,
		Limit:          params.Limit,
	})
}
