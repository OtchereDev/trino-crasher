package config

import "time"

// StoreConfig holds configuration for the in-memory store
type StoreConfig struct {
	// PostgreSQL connection strings (multiple databases)
	KetoDSN    string // Keto permissions database (keto_relation_tuples)
	AuthDBDSN  string // Auth database (group_relations)
	AssetDBDSN string // Asset database (devices)

	// MongoDB connection string
	MongoDSN string

	// MongoDB database name
	MongoDatabase string

	// Refresh intervals
	IdentityRefreshInterval time.Duration
	DeviceRefreshInterval   time.Duration

	// Batch sizes for loading data
	PostgresBatchSize int
	MongoBatchSize    int

	// Enable debug logging for data loading
	DebugLogging bool
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *StoreConfig {
	return &StoreConfig{
		IdentityRefreshInterval: 5 * time.Minute,
		DeviceRefreshInterval:   5 * time.Minute,
		PostgresBatchSize:       10000,
		MongoBatchSize:          10000,
		DebugLogging:            false,
	}
}
