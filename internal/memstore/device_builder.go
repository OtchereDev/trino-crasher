package memstore

import (
	"github.com/OtchereDev/trino-crasher/pkg/logger"
)

// BuildDeviceStore builds the device store by joining PostgreSQL and MongoDB data
// This replicates the device search SQL query from the original Trino implementation
func BuildDeviceStore(pgData *PostgresData, mongoIdentities map[string]MongoIdentity, logger logger.Logger) *DeviceStore {
	store := NewDeviceStore()

	logger.Debug("Building device store from loaded data...")

	// Step 1: Build user_devices mapping (user_email -> []device_id)
	userDevices := make(map[string][]string) // user_email -> []device_id

	for userEmail, groupIDs := range pgData.UserGroups {
		deviceIDs := []string{}

		for _, groupID := range groupIDs {
			members, ok := pgData.GroupMembers[groupID]
			if !ok {
				continue
			}

			for _, member := range members {
				if member.Type == "things" {
					deviceIDs = append(deviceIDs, member.MemberID)
				}
			}
		}

		if len(deviceIDs) > 0 {
			userDevices[userEmail] = deviceIDs
		}
	}

	logger.Debug("Step 1: Built user_devices mapping")

	// Step 2: Build device_links (user_email + dfx_device + device_id)
	type DeviceLink struct {
		UserEmail string
		DFXDevice string
		DeviceID  string
	}

	deviceLinks := []DeviceLink{}

	for userEmail, deviceIDs := range userDevices {
		for _, deviceID := range deviceIDs {
			if dfxDevice, ok := pgData.DeviceLinks[deviceID]; ok {
				deviceLinks = append(deviceLinks, DeviceLink{
					UserEmail: userEmail,
					DFXDevice: dfxDevice,
					DeviceID:  deviceID,
				})
			}
		}
	}

	logger.Debug("Step 2: Built device_links")

	// Step 3: Join with device details from asset-db
	type DeviceDetail struct {
		UserEmail      string
		DeviceID       string
		DFXTag         string
		IdentityID     string
		DeviceName     string
		DeviceLastSeen string
	}

	deviceDetails := []DeviceDetail{}

	for _, link := range deviceLinks {
		detail := DeviceDetail{
			UserEmail: link.UserEmail,
			DeviceID:  link.DFXDevice, // Use dfx_device as device_id
		}

		// Get device info from asset-db
		if deviceInfo, ok := pgData.Devices[link.DFXDevice]; ok {
			detail.DFXTag = deviceInfo.AssetID
			detail.IdentityID = deviceInfo.IdentityID
			detail.DeviceName = deviceInfo.Name
			detail.DeviceLastSeen = deviceInfo.LastSeen
		}

		deviceDetails = append(deviceDetails, detail)
	}

	logger.Debug("Step 3: Joined with device details")

	// Step 4: Join with identity details from MongoDB
	recordsAdded := 0

	for _, detail := range deviceDetails {
		// Skip records without device_id (WHERE device_id IS NOT NULL)
		if detail.DeviceID == "" {
			continue
		}

		rec := DeviceRecord{
			UserEmail:      detail.UserEmail,
			DeviceID:       detail.DeviceID,
			DFXTag:         detail.DFXTag,
			IdentityID:     detail.IdentityID,
			DeviceName:     detail.DeviceName,
			DeviceLastSeen: detail.DeviceLastSeen,
		}

		// Get identity info from MongoDB
		if detail.IdentityID != "" {
			if mongoIdentity, ok := mongoIdentities[detail.IdentityID]; ok {
				rec.AssetTag = mongoIdentity.AssetTag
				rec.IdentityDeviceName = mongoIdentity.DeviceName
				rec.SiteName = mongoIdentity.SiteName
				rec.DeviceAddress = mongoIdentity.DeviceAddress
				rec.WhatThreeWords = mongoIdentity.WhatThreeWords
			}
		}

		// COALESCE empty strings for fields that might be NULL
		if rec.IdentityDeviceName == "" {
			rec.IdentityDeviceName = ""
		}
		if rec.SiteName == "" {
			rec.SiteName = ""
		}
		if rec.DeviceAddress == "" {
			rec.DeviceAddress = ""
		}
		if rec.WhatThreeWords == "" {
			rec.WhatThreeWords = ""
		}

		store.AddDeviceRecord(rec)
		recordsAdded++
	}

	logger.Debug("Step 4: Joined with identity details")
	logger.Debug("Device store built successfully with", recordsAdded, "records")

	return store
}
