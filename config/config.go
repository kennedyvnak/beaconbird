package config

import "time"

// Config holds environment-derived runtime settings for API and worker binaries.
type Config struct {
	DatabaseURL               string
	APIPort                   string
	AWSRegion                 string
	AWSAccessKeyID            string
	AWSSecretAccessKey        string
	SESFromEmail              string
	FCMCredentialsJSON        string
	WorkerID                  string
	WorkerPollInterval        time.Duration
	WorkerConcurrency         int
	WorkerHeartbeatInterval   time.Duration
	WorkerStaleThreshold      time.Duration
	WorkerMaxRetries          int
	WorkerBaseDelay           time.Duration
	WorkerMaxDelay            time.Duration
}
