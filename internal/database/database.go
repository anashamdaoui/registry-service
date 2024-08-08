// internal/database/database.go

package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"registry-service/internal/middleware"
)

type MongoDB struct {
	client     *mongo.Client
	collection *mongo.Collection
}

// NewMongoDB creates a new MongoDB instance
func NewMongoDB(uri string, dbName string, collectionName string) (*MongoDB, error) {
	logger := middleware.GetLogger()

	logger.Debug("DB - ", "Connecting to MongoDB at URI: %s", uri)

	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	logger.Debug("DB - ", "Connected to MongoDB. Using database: %s, collection: %s", dbName, collectionName)
	collection := client.Database(dbName).Collection(collectionName)

	return &MongoDB{
		client:     client,
		collection: collection,
	}, nil
}

// Disconnect closes the connection to the MongoDB database
func (db *MongoDB) Disconnect() error {
	logger := middleware.GetLogger()

	logger.Debug("DB - ", "Disconnecting from MongoDB...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := db.client.Disconnect(ctx)
	if err != nil {
		logger.Info("DB - ", "Failed to disconnect from MongoDB: %v", err)
	} else {
		logger.Debug("DB - ", "Disconnected from MongoDB successfully.")
	}
	return err
}

// CreateIndexes ensures the necessary indexes are created in the MongoDB collection
// Creates an index on the address to avoid having duplicate entries for the same address
func (db *MongoDB) CreateIndexes() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger := middleware.GetLogger()

	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "address", Value: 1}},
		Options: options.Index().SetUnique(true),
	}

	_, err := db.collection.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		logger.Info("", "Failed to create index on MongoDB collection: %v", err)
		return err
	}

	logger.Info("", "Index created successfully on MongoDB collection.")
	return nil
}

// InsertWorker inserts a new worker into the collection
func (db *MongoDB) InsertWorker(address string) error {
	logger := middleware.GetLogger()

	logger.Debug("DB - ", "Inserting new worker with address: %s", address)

	worker := bson.M{
		"address":           address,
		"is_healthy":        true,
		"last_health_check": time.Now(),
	}
	_, err := db.collection.InsertOne(context.TODO(), worker)
	if err != nil {
		logger.Info("DB - ", "Failed to insert worker: %v", err)
	} else {
		logger.Debug("DB - ", "Worker inserted successfully with address: %s", address)
	}
	return err
}

// UpdateWorkerHealth updates the health status of a worker
func (db *MongoDB) UpdateWorkerHealth(address string, isHealthy bool) error {
	logger := middleware.GetLogger()

	logger.Debug("DB - ", "Updating health for worker with address: %s", address)

	filter := bson.M{"address": address}
	update := bson.M{
		"$set": bson.M{
			"is_healthy":        isHealthy,
			"last_health_check": time.Now(),
		},
	}
	_, err := db.collection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		logger.Info("DB - ", "Failed to update worker health: %v", err)
	} else {
		logger.Debug("DB - ", "Worker health updated successfully for address: %s", address)
	}
	return err
}

// GetAllWorkers retrieves all workers from the collection
func (db *MongoDB) GetAllWorkers() ([]bson.M, error) {
	logger := middleware.GetLogger()

	logger.Debug("DB - ", "Retrieving all workers from MongoDB collection.")

	var workers []bson.M
	cursor, err := db.collection.Find(context.TODO(), bson.M{})
	if err != nil {
		logger.Info("DB - ", "Failed to retrieve workers: %v", err)
		return nil, err
	}
	if err = cursor.All(context.TODO(), &workers); err != nil {
		logger.Info("DB - ", "Failed to decode workers: %v", err)
		return nil, err
	}
	logger.Debug("DB - ", "Retrieved %d workers from MongoDB.", len(workers))
	return workers, nil
}

// ClearCollection clears all documents in the collection for testing purposes
func (db *MongoDB) ClearCollection() error {
	logger := middleware.GetLogger()

	logger.Debug("DB - ", "Clearing MongoDB collection for testing.")
	_, err := db.collection.DeleteMany(context.TODO(), bson.M{})
	if err != nil {
		logger.Info("DB - ", "Failed to clear collection: %v", err)
	} else {
		logger.Debug("DB - ", "MongoDB collection cleared successfully.")
	}
	return err
}

// DeleteWorker removes a worker from the MongoDB collection
func (db *MongoDB) DeleteWorker(address string) error {
	logger := middleware.GetLogger()
	logger.Debug("DB - ", "Removing worker with address %v", address)
	filter := bson.M{"address": address}
	_, err := db.collection.DeleteOne(context.TODO(), filter)
	if err != nil {
		logger.Info("DB - ", "Failed to delete worker from database: %v", err)
	}
	return err
}
