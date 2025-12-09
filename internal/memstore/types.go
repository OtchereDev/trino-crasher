package memstore

// IdentityRecord represents a single identity record in memory
// Fields are optimized for memory efficiency (order matters for struct alignment)
type IdentityRecord struct {
	UserEmail       string
	DFXDevice       string
	DeviceID        string
	IdentityID      string
	AssetTag        string
	DeviceName      string
	SiteName        string
	DeviceAddress   string
	WhatThreeWords  string

	// Pre-lowercased fields for fast substring search
	searchableText string // Concatenated lowercase search fields
}

// DeviceRecord represents a single device record in memory
type DeviceRecord struct {
	UserEmail      string
	DeviceID       string
	DFXTag         string
	IdentityID     string
	DeviceName     string
	DeviceLastSeen string
	AssetTag       string
	IdentityDeviceName string
	SiteName       string
	DeviceAddress  string
	WhatThreeWords string

	// Pre-lowercased fields for fast substring search
	searchableText string // Concatenated lowercase search fields
}

// DeviceInfo represents device information for identity search results
type DeviceInfo struct {
	DFXDevice string
	DeviceID  string
}

// IdentitySearchResult matches the repo layer's expected structure
type IdentitySearchResult struct {
	IdentityID string
	AssetTag   string
	Devices    []DeviceInfo
}

// DeviceSearchResult matches the repo layer's expected structure
type DeviceSearchResult struct {
	DeviceID       string
	DeviceLastSeen string
	DFXTag         string
	IdentityID     string
	DeviceName     string
	AssetTag       string
}

// IdentityStore holds all identity records with indexes for fast querying
type IdentityStore struct {
	// All records stored as a slice for compact memory
	records []IdentityRecord

	// Index: user email -> slice of record indices
	userIndex map[string][]int
}

// DeviceStore holds all device records with indexes for fast querying
type DeviceStore struct {
	// All records stored as a slice for compact memory
	records []DeviceRecord

	// Index: user email -> slice of record indices
	userIndex map[string][]int
}

// NewIdentityStore creates an empty identity store
func NewIdentityStore() *IdentityStore {
	return &IdentityStore{
		records:   make([]IdentityRecord, 0, 10000),
		userIndex: make(map[string][]int),
	}
}

// NewDeviceStore creates an empty device store
func NewDeviceStore() *DeviceStore {
	return &DeviceStore{
		records:   make([]DeviceRecord, 0, 10000),
		userIndex: make(map[string][]int),
	}
}

// AddIdentityRecord adds a record and updates indexes
func (s *IdentityStore) AddIdentityRecord(rec IdentityRecord) {
	idx := len(s.records)
	s.records = append(s.records, rec)
	s.userIndex[rec.UserEmail] = append(s.userIndex[rec.UserEmail], idx)
}

// AddDeviceRecord adds a record and updates indexes
func (s *DeviceStore) AddDeviceRecord(rec DeviceRecord) {
	idx := len(s.records)
	s.records = append(s.records, rec)
	s.userIndex[rec.UserEmail] = append(s.userIndex[rec.UserEmail], idx)
}

// Size returns the number of records in the identity store
func (s *IdentityStore) Size() int {
	return len(s.records)
}

// Size returns the number of records in the device store
func (s *DeviceStore) Size() int {
	return len(s.records)
}
