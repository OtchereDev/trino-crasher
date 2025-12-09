package memstore

import (
	"github.com/OtchereDev/trino-crasher/pkg/logger"
)

// BuildIdentityStore builds the identity store by joining PostgreSQL and MongoDB data
// This replicates the complex SQL query from the original Trino implementation
func BuildIdentityStore(pgData *PostgresData, mongoIdentities map[string]MongoIdentity, logger logger.Logger) *IdentityStore {
	store := NewIdentityStore()

	logger.Debug("Building identity store from loaded data...")

	// Step 1: Build user_identities mapping (user_email -> []identity_id)
	userIdentities := make(map[string]map[string]bool) // user_email -> set of identity_ids

	for userEmail, groupIDs := range pgData.UserGroups {
		identitySet := make(map[string]bool)

		for _, groupID := range groupIDs {
			members, ok := pgData.GroupMembers[groupID]
			if !ok {
				continue
			}

			for _, member := range members {
				if member.Type == "identities" {
					identitySet[member.MemberID] = true
				}
			}
		}

		if len(identitySet) > 0 {
			userIdentities[userEmail] = identitySet
		}
	}

	logger.Debug("Step 1: Built user_identities mapping")

	// Step 2: Build user_devices mapping (user_email -> []device_id)
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

	logger.Debug("Step 2: Built user_devices mapping")

	// Step 3: Build device_links_with_user (user_email + dfx_device + device_id)
	type DeviceLink struct {
		UserEmail string
		DFXDevice string
		DeviceID  string
	}

	deviceLinksWithUser := []DeviceLink{}

	for userEmail, deviceIDs := range userDevices {
		for _, deviceID := range deviceIDs {
			if dfxDevice, ok := pgData.DeviceLinks[deviceID]; ok {
				deviceLinksWithUser = append(deviceLinksWithUser, DeviceLink{
					UserEmail: userEmail,
					DFXDevice: dfxDevice,
					DeviceID:  deviceID,
				})
			}
		}
	}

	logger.Debug("Step 3: Built device_links_with_user")

	// Step 4: Build identity_assignments (includes identity_id)
	type IdentityAssignment struct {
		UserEmail  string
		DFXDevice  string
		DeviceID   string
		IdentityID string
	}

	identityAssignments := []IdentityAssignment{}

	for _, link := range deviceLinksWithUser {
		identityID := ""
		if assignedID, ok := pgData.IdentityAssignments[link.DeviceID]; ok {
			identityID = assignedID
		}

		identityAssignments = append(identityAssignments, IdentityAssignment{
			UserEmail:  link.UserEmail,
			DFXDevice:  link.DFXDevice,
			DeviceID:   link.DeviceID,
			IdentityID: identityID,
		})
	}

	logger.Debug("Step 4: Built identity_assignments")

	// Step 5: Add records from identity_assignments with MongoDB data
	recordsAdded := 0

	for _, assignment := range identityAssignments {
		if assignment.IdentityID == "" {
			continue
		}

		mongoIdentity, hasMongoData := mongoIdentities[assignment.IdentityID]

		rec := IdentityRecord{
			UserEmail:  assignment.UserEmail,
			DFXDevice:  assignment.DFXDevice,
			DeviceID:   assignment.DeviceID,
			IdentityID: assignment.IdentityID,
		}

		if hasMongoData {
			rec.AssetTag = mongoIdentity.AssetTag
			rec.DeviceName = mongoIdentity.DeviceName
			rec.SiteName = mongoIdentity.SiteName
			rec.DeviceAddress = mongoIdentity.DeviceAddress
			rec.WhatThreeWords = mongoIdentity.WhatThreeWords
		}

		store.AddIdentityRecord(rec)
		recordsAdded++
	}

	logger.Debug("Step 5: Added records from identity_assignments")

	// Step 6: Add records from all_identities_with_user (FULL OUTER JOIN behavior)
	// Add identities that may not have device assignments
	for userEmail, identitySet := range userIdentities {
		for identityID := range identitySet {
			mongoIdentity, hasMongoData := mongoIdentities[identityID]

			rec := IdentityRecord{
				UserEmail:  userEmail,
				IdentityID: identityID,
			}

			if hasMongoData {
				rec.AssetTag = mongoIdentity.AssetTag
				rec.DeviceName = mongoIdentity.DeviceName
				rec.SiteName = mongoIdentity.SiteName
				rec.DeviceAddress = mongoIdentity.DeviceAddress
				rec.WhatThreeWords = mongoIdentity.WhatThreeWords
			}

			store.AddIdentityRecord(rec)
			recordsAdded++
		}
	}

	logger.Debug("Step 6: Added records from all_identities_with_user")
	logger.Debug("Identity store built successfully with", recordsAdded, "records")

	return store
}
