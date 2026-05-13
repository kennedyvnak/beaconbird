package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kennedyvnak/beaconbird/config"
	"github.com/kennedyvnak/beaconbird/internal/repository"
	"github.com/kennedyvnak/beaconbird/internal/repository/sqlc"
	"github.com/kennedyvnak/beaconbird/internal/worker"
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

	startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(startupCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(startupCtx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	queries := sqlc.New(pool)
	jobRepo := repository.NewDeliveryJobRepo(pool, queries)
	attemptRepo := repository.NewDeliveryAttemptRepo(queries)
	txRunner := repository.NewTxRunner(pool)

	registry, err := worker.BuildProviderRegistry(startupCtx, cfg, worker.ProviderFactories{})
	if err != nil {
		return err
	}

	scheduler := worker.NewScheduler(
		jobRepo,
		cfg.WorkerBaseDelay,
		cfg.WorkerMaxDelay,
		cfg.WorkerStaleThreshold,
		time.Minute,
	)
	executor := worker.NewExecutor(registry, jobRepo, attemptRepo, scheduler, txRunner)
	heartbeat := worker.NewHeartbeat(jobRepo, cfg.WorkerID, cfg.WorkerHeartbeatInterval)
	poller := worker.NewPoller(jobRepo, executor, cfg.WorkerID, cfg.WorkerConcurrency, cfg.WorkerPollInterval)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go heartbeat.Run(ctx)
	go scheduler.RunRecovery(ctx)

	log.Printf("worker %s running", cfg.WorkerID)
	poller.Run(ctx)
	log.Println("worker shutdown complete")
	return nil
}
