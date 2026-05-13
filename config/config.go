package config

import (
	"crypto/rand"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL             string
	APIPort                 string
	AWSRegion               string
	AWSAccessKeyID          string
	AWSSecretAccessKey      string
	SESFromEmail            string
	FCMCredentialsJSON      string
	WorkerID                string
	WorkerPollInterval      time.Duration
	WorkerConcurrency       int
	WorkerHeartbeatInterval time.Duration
	WorkerStaleThreshold    time.Duration
	WorkerMaxRetries        int
	WorkerBaseDelay         time.Duration
	WorkerMaxDelay          time.Duration
}

func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("config: DATABASE_URL is required")
	}

	workerID := os.Getenv("WORKER_ID")
	if workerID == "" {
		workerID = newUUID()
	}

	return &Config{
		DatabaseURL:             dbURL,
		APIPort:                 envOrDefault("API_PORT", "8080"),
		AWSRegion:               os.Getenv("AWS_REGION"),
		AWSAccessKeyID:          os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretAccessKey:      os.Getenv("AWS_SECRET_ACCESS_KEY"),
		SESFromEmail:            os.Getenv("SES_FROM_EMAIL"),
		FCMCredentialsJSON:      envOrDefault("FCM_CREDENTIALS_JSON", "./credentials/fcm.json"),
		WorkerID:                workerID,
		WorkerPollInterval:      envDuration("WORKER_POLL_INTERVAL", 5*time.Second),
		WorkerConcurrency:       envInt("WORKER_CONCURRENCY", 10),
		WorkerHeartbeatInterval: envDuration("WORKER_HEARTBEAT_INTERVAL", 30*time.Second),
		WorkerStaleThreshold:    envDuration("WORKER_STALE_THRESHOLD", 2*time.Minute),
		WorkerMaxRetries:        envInt("WORKER_MAX_RETRIES", 3),
		WorkerBaseDelay:         envDuration("WORKER_BASE_DELAY", 30*time.Second),
		WorkerMaxDelay:          envDuration("WORKER_MAX_DELAY", time.Hour),
	}, nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
