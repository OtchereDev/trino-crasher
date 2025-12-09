package memstore

import (
	"context"
	"fmt"

	"github.com/OtchereDev/trino-crasher/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresData holds all data loaded from PostgreSQL
type PostgresData struct {
	// User profiles: user_id -> user_email
	UserProfiles map[string]string

	// User groups: user_email -> []group_id
	UserGroups map[string][]string

	// Group members: group_id -> []member_id with type
	GroupMembers map[string][]GroupMember

	// Device links: device_id -> dfx_device
	DeviceLinks map[string]string

	// Identity assignments: device_id -> identity_id
	IdentityAssignments map[string]string

	// Devices info from asset-db
	Devices map[string]DeviceInfo
}

// GroupMember represents a member of a group
type GroupMember struct {
	MemberID string
	Type     string // 'identities' or 'things'
}

// DeviceInfo represents device information from asset-db
type DeviceInfo struct {
	ID         string
	AssetID    string
	IdentityID string
	Name       string
	LastSeen   string
}

// LoadPostgresData loads all necessary data from PostgreSQL databases
func LoadPostgresData(ctx context.Context, pool *pgxpool.Pool, logger logger.Logger) (*PostgresData, error) {
	data := &PostgresData{
		UserProfiles:        make(map[string]string),
		UserGroups:          make(map[string][]string),
		GroupMembers:        make(map[string][]GroupMember),
		DeviceLinks:         make(map[string]string),
		IdentityAssignments: make(map[string]string),
		Devices:             make(map[string]DeviceInfo),
	}

	// Load user profiles
	logger.Debug("Loading user profiles from PostgreSQL...")
	if err := loadUserProfiles(ctx, pool, data); err != nil {
		return nil, fmt.Errorf("failed to load user profiles: %w", err)
	}
	logger.Debug(fmt.Sprintf("Loaded %d user profiles", len(data.UserProfiles)))

	// Load user groups
	logger.Debug("Loading user groups from PostgreSQL...")
	if err := loadUserGroups(ctx, pool, data); err != nil {
		return nil, fmt.Errorf("failed to load user groups: %w", err)
	}
	logger.Debug(fmt.Sprintf("Loaded user groups for %d users", len(data.UserGroups)))

	// Load group members
	logger.Debug("Loading group members from PostgreSQL...")
	if err := loadGroupMembers(ctx, pool, data); err != nil {
		return nil, fmt.Errorf("failed to load group members: %w", err)
	}
	logger.Debug(fmt.Sprintf("Loaded members for %d groups", len(data.GroupMembers)))

	// Load device links
	logger.Debug("Loading device links from PostgreSQL...")
	if err := loadDeviceLinks(ctx, pool, data); err != nil {
		return nil, fmt.Errorf("failed to load device links: %w", err)
	}
	logger.Debug(fmt.Sprintf("Loaded %d device links", len(data.DeviceLinks)))

	// Load identity assignments
	logger.Debug("Loading identity assignments from PostgreSQL...")
	if err := loadIdentityAssignments(ctx, pool, data); err != nil {
		return nil, fmt.Errorf("failed to load identity assignments: %w", err)
	}
	logger.Debug(fmt.Sprintf("Loaded %d identity assignments", len(data.IdentityAssignments)))

	// Load devices from asset-db
	logger.Debug("Loading devices from asset-db...")
	if err := loadDevices(ctx, pool, data); err != nil {
		logger.Warn(fmt.Sprintf("Failed to load devices (may not exist yet): %v", err))
		// Don't fail if devices don't exist yet
	} else {
		logger.Debug(fmt.Sprintf("Loaded %d devices", len(data.Devices)))
	}

	return data, nil
}

// loadUserProfiles loads user_id -> user_email mappings
func loadUserProfiles(ctx context.Context, pool *pgxpool.Pool, data *PostgresData) error {
	query := `
		SELECT subject, object
		FROM postgresql.public.keto_0000000000_relation_tuples
		WHERE relation = 'user_profile'
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var userID, userEmail string
		if err := rows.Scan(&userID, &userEmail); err != nil {
			return err
		}
		data.UserProfiles[userID] = userEmail
	}

	return rows.Err()
}

// loadUserGroups loads user_email -> []group_id mappings
func loadUserGroups(ctx context.Context, pool *pgxpool.Pool, data *PostgresData) error {
	query := `
		SELECT subject, object
		FROM postgresql.public.keto_0000000000_relation_tuples
		WHERE relation = 'member'
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var userID, groupID string
		if err := rows.Scan(&userID, &groupID); err != nil {
			return err
		}

		// Convert userID to userEmail
		if userEmail, ok := data.UserProfiles[userID]; ok {
			data.UserGroups[userEmail] = append(data.UserGroups[userEmail], groupID)
		}
	}

	return rows.Err()
}

// loadGroupMembers loads group_id -> []member mappings
func loadGroupMembers(ctx context.Context, pool *pgxpool.Pool, data *PostgresData) error {
	query := `
		SELECT group_id, member_id, type
		FROM "auth-db".public.group_relations
		WHERE type IN ('identities', 'things')
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var groupID, memberID, memberType string
		if err := rows.Scan(&groupID, &memberID, &memberType); err != nil {
			return err
		}

		data.GroupMembers[groupID] = append(data.GroupMembers[groupID], GroupMember{
			MemberID: memberID,
			Type:     memberType,
		})
	}

	return rows.Err()
}

// loadDeviceLinks loads device_id -> dfx_device mappings
func loadDeviceLinks(ctx context.Context, pool *pgxpool.Pool, data *PostgresData) error {
	query := `
		SELECT subject, object
		FROM postgresql.public.keto_0000000000_relation_tuples
		WHERE relation = 'idLink'
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var dfxDevice, deviceID string
		if err := rows.Scan(&dfxDevice, &deviceID); err != nil {
			return err
		}
		data.DeviceLinks[deviceID] = dfxDevice
	}

	return rows.Err()
}

// loadIdentityAssignments loads device_id -> identity_id mappings
func loadIdentityAssignments(ctx context.Context, pool *pgxpool.Pool, data *PostgresData) error {
	query := `
		SELECT subject, object
		FROM postgresql.public.keto_0000000000_relation_tuples
		WHERE relation = 'assignedDevice'
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var deviceID, identityID string
		if err := rows.Scan(&deviceID, &identityID); err != nil {
			return err
		}
		data.IdentityAssignments[deviceID] = identityID
	}

	return rows.Err()
}

// loadDevices loads device information from asset-db
func loadDevices(ctx context.Context, pool *pgxpool.Pool, data *PostgresData) error {
	query := `
		SELECT id, asset_id, identity_id, name, last_seen
		FROM "asset-db".public.devices
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var dev DeviceInfo
		var assetID, identityID, name, lastSeen *string

		if err := rows.Scan(&dev.ID, &assetID, &identityID, &name, &lastSeen); err != nil {
			return err
		}

		if assetID != nil {
			dev.AssetID = *assetID
		}
		if identityID != nil {
			dev.IdentityID = *identityID
		}
		if name != nil {
			dev.Name = *name
		}
		if lastSeen != nil {
			dev.LastSeen = *lastSeen
		}

		data.Devices[dev.ID] = dev
	}

	return rows.Err()
}
