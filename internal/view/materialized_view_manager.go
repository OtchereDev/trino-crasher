package view

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OtchereDev/trino-crasher/internal/memstore"
	"github.com/OtchereDev/trino-crasher/pkg/config"
	"github.com/OtchereDev/trino-crasher/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MaterializedViewManager manages in-memory stores with blue-green deployment
type MaterializedViewManager struct {
	logger logger.Logger
	config *config.StoreConfig

	// Database connections (3 PostgreSQL databases + 1 MongoDB)
	ketoPool    *pgxpool.Pool // Keto permissions database
	authPool    *pgxpool.Pool // Auth database
	assetPool   *pgxpool.Pool // Asset database
	mongoClient *mongo.Client // MongoDB

	// Blue-Green stores for identities
	identityStoreBlue  atomic.Pointer[memstore.IdentityStore]
	identityStoreGreen atomic.Pointer[memstore.IdentityStore]
	activeIdentityBlue atomic.Bool // true = blue active, false = green active

	// Blue-Green stores for devices
	deviceStoreBlue  atomic.Pointer[memstore.DeviceStore]
	deviceStoreGreen atomic.Pointer[memstore.DeviceStore]
	activeDeviceBlue atomic.Bool // true = blue active, false = green active

	// Refresh control
	identityRefreshMu sync.Mutex
	deviceRefreshMu   sync.Mutex
}

// NewMaterializedViewManager creates a new manager with database connections
func NewMaterializedViewManager(cfg *config.StoreConfig, logger logger.Logger) (*MaterializedViewManager, error) {
	mvm := &MaterializedViewManager{
		logger: logger,
		config: cfg,
	}

	// Initialize Keto PostgreSQL connection pool
	logger.Debug("Connecting to Keto database...")
	ketoPool, err := pgxpool.New(context.Background(), cfg.KetoDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Keto database: %w", err)
	}
	mvm.ketoPool = ketoPool
	logger.Debug("✓ Connected to Keto database")

	// Initialize Auth PostgreSQL connection pool
	logger.Debug("Connecting to Auth database...")
	authPool, err := pgxpool.New(context.Background(), cfg.AuthDBDSN)
	if err != nil {
		ketoPool.Close()
		return nil, fmt.Errorf("failed to connect to Auth database: %w", err)
	}
	mvm.authPool = authPool
	logger.Debug("✓ Connected to Auth database")

	// Initialize Asset PostgreSQL connection pool
	logger.Debug("Connecting to Asset database...")
	assetPool, err := pgxpool.New(context.Background(), cfg.AssetDBDSN)
	if err != nil {
		ketoPool.Close()
		authPool.Close()
		return nil, fmt.Errorf("failed to connect to Asset database: %w", err)
	}
	mvm.assetPool = assetPool
	logger.Debug("✓ Connected to Asset database")

	// Initialize MongoDB connection
	logger.Debug("Connecting to MongoDB...")
	mongoClient, err := mongo.Connect(context.Background(), options.Client().ApplyURI(cfg.MongoDSN))
	if err != nil {
		ketoPool.Close()
		authPool.Close()
		assetPool.Close()
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping MongoDB to verify connection
	if err := mongoClient.Ping(context.Background(), nil); err != nil {
		ketoPool.Close()
		authPool.Close()
		assetPool.Close()
		mongoClient.Disconnect(context.Background())
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}
	mvm.mongoClient = mongoClient
	logger.Debug("✓ Connected to MongoDB")

	// Start with blue as active
	mvm.activeIdentityBlue.Store(true)
	mvm.activeDeviceBlue.Store(true)

	return mvm, nil
}

// Close closes all database connections
func (mvm *MaterializedViewManager) Close() error {
	mvm.logger.Debug("Closing database connections...")

	if mvm.ketoPool != nil {
		mvm.ketoPool.Close()
	}

	if mvm.authPool != nil {
		mvm.authPool.Close()
	}

	if mvm.assetPool != nil {
		mvm.assetPool.Close()
	}

	if mvm.mongoClient != nil {
		if err := mvm.mongoClient.Disconnect(context.Background()); err != nil {
			return err
		}
	}

	mvm.logger.Debug("✓ Database connections closed")
	return nil
}

// InitializeTables loads initial data and creates both blue and green stores
func (mvm *MaterializedViewManager) InitializeTables(ctx context.Context) error {
	mvm.logger.Debug("Initializing in-memory stores...")

	// Check if MongoDB identities exist
	hasIdentities, err := memstore.CheckMongoDBIdentitiesExists(ctx, mvm.mongoClient, mvm.config.MongoDatabase)
	if err != nil {
		mvm.logger.Error(fmt.Sprintf("Failed to check MongoDB identities: %v", err))
		hasIdentities = false
	}

	// Load PostgreSQL data from all three databases
	mvm.logger.Debug("Loading data from PostgreSQL databases...")
	pgData, err := memstore.LoadPostgresData(ctx, mvm.ketoPool, mvm.authPool, mvm.assetPool, mvm.logger)
	if err != nil {
		return fmt.Errorf("failed to load PostgreSQL data: %w", err)
	}

	// Load MongoDB identities
	var mongoIdentities map[string]memstore.MongoIdentity
	if hasIdentities {
		mvm.logger.Debug("Loading identities from MongoDB...")
		mongoIdentities, err = memstore.LoadMongoIdentities(ctx, mvm.mongoClient, mvm.config.MongoDatabase, mvm.logger)
		if err != nil {
			return fmt.Errorf("failed to load MongoDB identities: %w", err)
		}
	} else {
		mvm.logger.Warn("⚠️ No identities found in MongoDB - creating empty stores")
		mongoIdentities = make(map[string]memstore.MongoIdentity)
	}

	// Build identity stores (blue and green)
	mvm.logger.Debug("Building identity stores...")
	identityBlue := memstore.BuildIdentityStore(pgData, mongoIdentities, mvm.logger)
	identityGreen := memstore.BuildIdentityStore(pgData, mongoIdentities, mvm.logger)
	mvm.identityStoreBlue.Store(identityBlue)
	mvm.identityStoreGreen.Store(identityGreen)
	mvm.logger.Debug(fmt.Sprintf("✓ Identity stores initialized (size: %d records)", identityBlue.Size()))

	// Build device stores (blue and green)
	mvm.logger.Debug("Building device stores...")
	deviceBlue := memstore.BuildDeviceStore(pgData, mongoIdentities, mvm.logger)
	deviceGreen := memstore.BuildDeviceStore(pgData, mongoIdentities, mvm.logger)
	mvm.deviceStoreBlue.Store(deviceBlue)
	mvm.deviceStoreGreen.Store(deviceGreen)
	mvm.logger.Debug(fmt.Sprintf("✓ Device stores initialized (size: %d records)", deviceBlue.Size()))

	mvm.logger.Debug("✓ All in-memory stores initialized successfully")
	return nil
}

// RefreshIdentityStore refreshes the inactive identity store and swaps it to active
func (mvm *MaterializedViewManager) RefreshIdentityStore(ctx context.Context) error {
	mvm.identityRefreshMu.Lock()
	defer mvm.identityRefreshMu.Unlock()

	start := time.Now()
	mvm.logger.Debug("Starting identity store refresh...")

	// Check if MongoDB identities exist
	hasIdentities, err := memstore.CheckMongoDBIdentitiesExists(ctx, mvm.mongoClient, mvm.config.MongoDatabase)
	if err != nil {
		mvm.logger.Error(fmt.Sprintf("Failed to check identities during refresh: %v", err))
		hasIdentities = false
	}

	if !hasIdentities {
		mvm.logger.Warn("⚠️ No identities found - skipping identity refresh")
		return nil
	}

	// Load fresh data from all three PostgreSQL databases
	mvm.logger.Debug("Loading fresh data from PostgreSQL databases...")
	pgData, err := memstore.LoadPostgresData(ctx, mvm.ketoPool, mvm.authPool, mvm.assetPool, mvm.logger)
	if err != nil {
		return fmt.Errorf("failed to load PostgreSQL data: %w", err)
	}

	mvm.logger.Debug("Loading fresh identities from MongoDB...")
	mongoIdentities, err := memstore.LoadMongoIdentities(ctx, mvm.mongoClient, mvm.config.MongoDatabase, mvm.logger)
	if err != nil {
		return fmt.Errorf("failed to load MongoDB identities: %w", err)
	}

	// Build new store
	mvm.logger.Debug("Building new identity store...")
	newStore := memstore.BuildIdentityStore(pgData, mongoIdentities, mvm.logger)

	// Atomic swap
	isBlueActive := mvm.activeIdentityBlue.Load()
	if isBlueActive {
		// Green is inactive, update it and swap
		mvm.identityStoreGreen.Store(newStore)
		mvm.activeIdentityBlue.Store(false)
		mvm.logger.Debug("✓ Swapped identity store: Blue -> Green")
	} else {
		// Blue is inactive, update it and swap
		mvm.identityStoreBlue.Store(newStore)
		mvm.activeIdentityBlue.Store(true)
		mvm.logger.Debug("✓ Swapped identity store: Green -> Blue")
	}

	duration := time.Since(start)
	mvm.logger.Debug(fmt.Sprintf("✓ Identity store refresh completed in %v (size: %d records)", duration, newStore.Size()))

	return nil
}

// RefreshDeviceStore refreshes the inactive device store and swaps it to active
func (mvm *MaterializedViewManager) RefreshDeviceStore(ctx context.Context) error {
	mvm.deviceRefreshMu.Lock()
	defer mvm.deviceRefreshMu.Unlock()

	start := time.Now()
	mvm.logger.Debug("Starting device store refresh...")

	// Load fresh data from all three PostgreSQL databases
	mvm.logger.Debug("Loading fresh data from PostgreSQL databases...")
	pgData, err := memstore.LoadPostgresData(ctx, mvm.ketoPool, mvm.authPool, mvm.assetPool, mvm.logger)
	if err != nil {
		return fmt.Errorf("failed to load PostgreSQL data: %w", err)
	}

	mvm.logger.Debug("Loading fresh identities from MongoDB...")
	mongoIdentities, err := memstore.LoadMongoIdentities(ctx, mvm.mongoClient, mvm.config.MongoDatabase, mvm.logger)
	if err != nil {
		// Don't fail if identities don't exist
		mvm.logger.Warn(fmt.Sprintf("Failed to load identities: %v", err))
		mongoIdentities = make(map[string]memstore.MongoIdentity)
	}

	// Build new store
	mvm.logger.Debug("Building new device store...")
	newStore := memstore.BuildDeviceStore(pgData, mongoIdentities, mvm.logger)

	// Atomic swap
	isBlueActive := mvm.activeDeviceBlue.Load()
	if isBlueActive {
		// Green is inactive, update it and swap
		mvm.deviceStoreGreen.Store(newStore)
		mvm.activeDeviceBlue.Store(false)
		mvm.logger.Debug("✓ Swapped device store: Blue -> Green")
	} else {
		// Blue is inactive, update it and swap
		mvm.deviceStoreBlue.Store(newStore)
		mvm.activeDeviceBlue.Store(true)
		mvm.logger.Debug("✓ Swapped device store: Green -> Blue")
	}

	duration := time.Since(start)
	mvm.logger.Debug(fmt.Sprintf("✓ Device store refresh completed in %v (size: %d records)", duration, newStore.Size()))

	return nil
}

// GetActiveIdentityStore returns the currently active identity store
func (mvm *MaterializedViewManager) GetActiveIdentityStore() *memstore.IdentityStore {
	if mvm.activeIdentityBlue.Load() {
		return mvm.identityStoreBlue.Load()
	}
	return mvm.identityStoreGreen.Load()
}

// GetActiveDeviceStore returns the currently active device store
func (mvm *MaterializedViewManager) GetActiveDeviceStore() *memstore.DeviceStore {
	if mvm.activeDeviceBlue.Load() {
		return mvm.deviceStoreBlue.Load()
	}
	return mvm.deviceStoreGreen.Load()
}

// StartPeriodicRefresh starts background goroutines for periodic refresh
func (mvm *MaterializedViewManager) StartPeriodicRefresh(ctx context.Context) {
	// Start identity store refresh
	mvm.logger.Debug(fmt.Sprintf("Starting periodic identity store refresh (interval: %v)", mvm.config.IdentityRefreshInterval))
	go func() {
		ticker := time.NewTicker(mvm.config.IdentityRefreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				mvm.logger.Debug("Periodic identity refresh triggered")
				if err := mvm.RefreshIdentityStore(ctx); err != nil {
					mvm.logger.Error(fmt.Sprintf("Periodic identity refresh failed: %v", err))
				} else {
					mvm.logger.Debug("Periodic identity refresh completed successfully")
				}
			case <-ctx.Done():
				mvm.logger.Debug("Stopping periodic identity store refresh")
				return
			}
		}
	}()

	// Start device store refresh
	mvm.logger.Debug(fmt.Sprintf("Starting periodic device store refresh (interval: %v)", mvm.config.DeviceRefreshInterval))
	go func() {
		ticker := time.NewTicker(mvm.config.DeviceRefreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				mvm.logger.Debug("Periodic device refresh triggered")
				if err := mvm.RefreshDeviceStore(ctx); err != nil {
					mvm.logger.Error(fmt.Sprintf("Periodic device refresh failed: %v", err))
				} else {
					mvm.logger.Debug("Periodic device refresh completed successfully")
				}
			case <-ctx.Done():
				mvm.logger.Debug("Stopping periodic device store refresh")
				return
			}
		}
	}()
}

// GetActiveIdentityStoreSize returns the size of the active identity store
func (mvm *MaterializedViewManager) GetActiveIdentityStoreSize() int {
	store := mvm.GetActiveIdentityStore()
	if store == nil {
		return 0
	}
	return store.Size()
}

// GetActiveDeviceStoreSize returns the size of the active device store
func (mvm *MaterializedViewManager) GetActiveDeviceStoreSize() int {
	store := mvm.GetActiveDeviceStore()
	if store == nil {
		return 0
	}
	return store.Size()
}
