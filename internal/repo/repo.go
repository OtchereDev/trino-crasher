package repo

import (
	"context"
	"fmt"

	"github.com/OtchereDev/trino-crasher/internal/view"
	"github.com/OtchereDev/trino-crasher/pkg/logger"
)

// Repo provides data access methods using the in-memory store
type Repo struct {
	logger                  logger.Logger
	materializedViewManager *view.MaterializedViewManager
}

// NewRepo creates a new repository instance
func NewRepo(viewManager *view.MaterializedViewManager, logger logger.Logger) *Repo {
	return &Repo{
		logger:                  logger,
		materializedViewManager: viewManager,
	}
}

// SearchIdentities searches for identities using the in-memory store
// This replaces the Trino SQL query with in-memory search
func (r *Repo) SearchIdentities(ctx context.Context, email, search string, isScan bool) ([]IdentitySearchResult, error) {
	r.logger.Debug(fmt.Sprintf("Searching identities for user: %s, search: %s, isScan: %v", email, search, isScan))

	// Get active identity store
	store := r.materializedViewManager.GetActiveIdentityStore()
	if store == nil {
		r.logger.Warn("Identity store is not initialized, returning empty results")
		return []IdentitySearchResult{}, nil
	}

	// Search in memory (limit 100 as per original query)
	results := store.SearchIdentities(email, search, isScan, 100)

	// Convert memstore results to repo results
	repoResults := make([]IdentitySearchResult, len(results))
	for i, result := range results {
		devices := make(DeviceInfos, len(result.Devices))
		for j, dev := range result.Devices {
			devices[j] = DeviceInfo{
				DFXDevice: dev.DFXDevice,
				DeviceID:  dev.DeviceID,
			}
		}

		repoResults[i] = IdentitySearchResult{
			IdentityID: result.IdentityID,
			AssetTag:   result.AssetTag,
			Devices:    devices,
		}
	}

	r.logger.Debug(fmt.Sprintf("Found %d identity results", len(repoResults)))
	return repoResults, nil
}

// SearchDevices searches for devices using the in-memory store with pagination
// This replaces the Trino SQL query with in-memory search
func (r *Repo) SearchDevices(ctx context.Context, email, search string, isScan bool, page, limit int) (
	results []DeviceSearchResult, total int, err error,
) {
	r.logger.Debug(fmt.Sprintf("Searching devices for user: %s, search: %s, isScan: %v, page: %d, limit: %d",
		email, search, isScan, page, limit))

	// Get active device store
	store := r.materializedViewManager.GetActiveDeviceStore()
	if store == nil {
		r.logger.Warn("Device store is not initialized, returning empty results")
		return []DeviceSearchResult{}, 0, nil
	}

	// Search in memory with pagination
	memResults, totalCount := store.SearchDevices(email, search, isScan, page, limit)

	// Convert memstore results to repo results
	repoResults := make([]DeviceSearchResult, len(memResults))
	for i, result := range memResults {
		repoResults[i] = DeviceSearchResult{
			DeviceID:       result.DeviceID,
			DeviceLastSeen: result.DeviceLastSeen,
			DFXTag:         result.DFXTag,
			IdentityID:     result.IdentityID,
			DeviceName:     result.DeviceName,
			AssetTag:       result.AssetTag,
		}
	}

	r.logger.Debug(fmt.Sprintf("Found %d device results (total: %d)", len(repoResults), totalCount))
	return repoResults, totalCount, nil
}
