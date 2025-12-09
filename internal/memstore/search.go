package memstore

import (
	"strings"
)

// SearchIdentities performs case-insensitive substring search on identity records
// This replaces the Trino SQL query with in-memory search
func (s *IdentityStore) SearchIdentities(userEmail, searchTerm string, isScan bool, limit int) []IdentitySearchResult {
	// Get indices for this user
	indices, ok := s.userIndex[userEmail]
	if !ok || len(indices) == 0 {
		return []IdentitySearchResult{}
	}

	searchLower := strings.ToLower(searchTerm)

	// Map to aggregate devices by identity_id
	identityMap := make(map[string]*IdentitySearchResult)

	for _, idx := range indices {
		rec := &s.records[idx]

		// Apply search filter
		if !matchesIdentitySearch(rec, searchLower, isScan) {
			continue
		}

		// Aggregate by identity_id
		if result, exists := identityMap[rec.IdentityID]; exists {
			// Add device to existing identity
			if rec.DFXDevice != "" && rec.DeviceID != "" {
				result.Devices = append(result.Devices, DeviceInfo{
					DFXDevice: rec.DFXDevice,
					DeviceID:  rec.DeviceID,
				})
			}
		} else {
			// Create new identity result
			result := &IdentitySearchResult{
				IdentityID: rec.IdentityID,
				AssetTag:   rec.AssetTag,
				Devices:    []DeviceInfo{},
			}

			if rec.DFXDevice != "" && rec.DeviceID != "" {
				result.Devices = append(result.Devices, DeviceInfo{
					DFXDevice: rec.DFXDevice,
					DeviceID:  rec.DeviceID,
				})
			}

			identityMap[rec.IdentityID] = result
		}

		// Early exit if we have enough results
		if len(identityMap) >= limit {
			break
		}
	}

	// Convert map to slice
	results := make([]IdentitySearchResult, 0, len(identityMap))
	for _, result := range identityMap {
		results = append(results, *result)
		if len(results) >= limit {
			break
		}
	}

	return results
}

// matchesIdentitySearch checks if a record matches the search criteria
func matchesIdentitySearch(rec *IdentityRecord, searchLower string, isScan bool) bool {
	if searchLower == "" {
		return true
	}

	if isScan {
		// Scan mode: only search dfx_device
		return strings.Contains(strings.ToLower(rec.DFXDevice), searchLower)
	}

	// Regular mode: search multiple fields
	return strings.Contains(strings.ToLower(rec.AssetTag), searchLower) ||
		strings.Contains(strings.ToLower(rec.DeviceName), searchLower) ||
		strings.Contains(strings.ToLower(rec.SiteName), searchLower) ||
		strings.Contains(strings.ToLower(rec.DeviceAddress), searchLower) ||
		strings.Contains(strings.ToLower(rec.WhatThreeWords), searchLower) ||
		strings.Contains(strings.ToLower(rec.DFXDevice), searchLower)
}

// SearchDevices performs case-insensitive substring search on device records with pagination
func (s *DeviceStore) SearchDevices(userEmail, searchTerm string, isScan bool, page, limit int) ([]DeviceSearchResult, int) {
	// Get indices for this user
	indices, ok := s.userIndex[userEmail]
	if !ok || len(indices) == 0 {
		return []DeviceSearchResult{}, 0
	}

	searchLower := strings.ToLower(searchTerm)

	// Collect matching records
	matches := make([]DeviceSearchResult, 0)

	for _, idx := range indices {
		rec := &s.records[idx]

		// Apply search filter
		if !matchesDeviceSearch(rec, searchLower, isScan) {
			continue
		}

		matches = append(matches, DeviceSearchResult{
			DeviceID:       rec.DeviceID,
			DeviceLastSeen: rec.DeviceLastSeen,
			DFXTag:         rec.DFXTag,
			IdentityID:     rec.IdentityID,
			DeviceName:     rec.DeviceName,
			AssetTag:       rec.AssetTag,
		})
	}

	total := len(matches)

	// Apply pagination
	skip := (page - 1) * limit
	if skip >= len(matches) {
		return []DeviceSearchResult{}, total
	}

	end := skip + limit
	if end > len(matches) {
		end = len(matches)
	}

	return matches[skip:end], total
}

// matchesDeviceSearch checks if a device record matches the search criteria
func matchesDeviceSearch(rec *DeviceRecord, searchLower string, isScan bool) bool {
	if searchLower == "" {
		return true
	}

	if isScan {
		// Scan mode: only search device_id
		return strings.Contains(strings.ToLower(rec.DeviceID), searchLower)
	}

	// Regular mode: search multiple fields
	return strings.Contains(strings.ToLower(rec.DeviceID), searchLower) ||
		strings.Contains(strings.ToLower(rec.DFXTag), searchLower) ||
		strings.Contains(strings.ToLower(rec.IdentityID), searchLower) ||
		strings.Contains(strings.ToLower(rec.DeviceName), searchLower) ||
		strings.Contains(strings.ToLower(rec.AssetTag), searchLower) ||
		strings.Contains(strings.ToLower(rec.IdentityDeviceName), searchLower) ||
		strings.Contains(strings.ToLower(rec.SiteName), searchLower) ||
		strings.Contains(strings.ToLower(rec.DeviceAddress), searchLower) ||
		strings.Contains(strings.ToLower(rec.WhatThreeWords), searchLower)
}
