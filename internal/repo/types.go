package repo

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// DeviceInfo represents device information in search results
type DeviceInfo struct {
	DFXDevice string `json:"dfxDevice"`
	DeviceID  string `json:"deviceId"`
}

// DeviceInfos is a slice of DeviceInfo that implements sql.Scanner
type DeviceInfos []DeviceInfo

// Scan implements sql.Scanner for DeviceInfos (for compatibility with existing code)
func (d *DeviceInfos) Scan(value interface{}) error {
	if value == nil {
		*d = []DeviceInfo{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal DeviceInfos: expected []byte, got %T", value)
	}

	var devices []DeviceInfo
	if err := json.Unmarshal(bytes, &devices); err != nil {
		return fmt.Errorf("failed to unmarshal DeviceInfos: %w", err)
	}

	*d = devices
	return nil
}

// Value implements driver.Valuer for DeviceInfos
func (d DeviceInfos) Value() (driver.Value, error) {
	if d == nil {
		return nil, nil
	}
	return json.Marshal(d)
}

// IdentitySearchResult represents a single identity search result
type IdentitySearchResult struct {
	IdentityID string      `json:"identity_id"`
	AssetTag   string      `json:"asset_tag"`
	Devices    DeviceInfos `json:"devices"`
}

// DeviceSearchResult represents a single device search result
type DeviceSearchResult struct {
	DeviceID       string `json:"device_id"`
	DeviceLastSeen string `json:"device_last_seen"`
	DFXTag         string `json:"dfx_tag"`
	IdentityID     string `json:"identity_id"`
	DeviceName     string `json:"device_name"`
	AssetTag       string `json:"asset_tag"`
}
