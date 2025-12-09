package memstore

import (
	"context"
	"fmt"

	"github.com/OtchereDev/trino-crasher/pkg/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// MongoIdentity represents an identity document from MongoDB
type MongoIdentity struct {
	ID             string `bson:"id"`
	AssetTag       string `bson:"assetTag"`
	DeviceName     string `bson:"deviceName"`
	SiteName       string `bson:"siteName"`
	DeviceAddress  string `bson:"deviceAddress"`
	WhatThreeWords string `bson:"whatThreeWords"`
}

// LoadMongoIdentities loads all identities from MongoDB
func LoadMongoIdentities(ctx context.Context, client *mongo.Client, dbName string, logger logger.Logger) (map[string]MongoIdentity, error) {
	identities := make(map[string]MongoIdentity)

	// Get the identities collection
	collection := client.Database(dbName).Collection("identities")

	// Check if collection exists and has documents
	count, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to count identities: %w", err)
	}

	if count == 0 {
		logger.Warn("No identities found in MongoDB")
		return identities, nil
	}

	logger.Debug(fmt.Sprintf("Loading %d identities from MongoDB...", count))

	// Query all identities
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to query identities: %w", err)
	}
	defer cursor.Close(ctx)

	// Iterate and load
	loadedCount := 0
	for cursor.Next(ctx) {
		var identity MongoIdentity
		if err := cursor.Decode(&identity); err != nil {
			logger.Warn(fmt.Sprintf("Failed to decode identity: %v", err))
			continue
		}

		identities[identity.ID] = identity
		loadedCount++
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	logger.Debug(fmt.Sprintf("Successfully loaded %d identities from MongoDB", loadedCount))
	return identities, nil
}

// CheckMongoDBIdentitiesExists checks if the identities collection has any documents
func CheckMongoDBIdentitiesExists(ctx context.Context, client *mongo.Client, dbName string) (bool, error) {
	collection := client.Database(dbName).Collection("identities")

	count, err := collection.CountDocuments(ctx, bson.M{}, nil)
	if err != nil {
		return false, fmt.Errorf("failed to check identities existence: %w", err)
	}

	return count > 0, nil
}
