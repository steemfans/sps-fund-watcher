package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/ety001/sps-fund-watcher/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	operationsCollection = "operations"
	syncStateCollection  = "sync_state"
)

// MongoDB represents a MongoDB storage client
type MongoDB struct {
	client     *mongo.Client
	database   *mongo.Database
	operations *mongo.Collection
	syncState  *mongo.Collection
}

// NewMongoDB creates a new MongoDB storage client
func NewMongoDB(uri, databaseName string) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Test connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db := client.Database(databaseName)

	return &MongoDB{
		client:     client,
		database:   db,
		operations: db.Collection(operationsCollection),
		syncState:  db.Collection(syncStateCollection),
	}, nil
}

// Close closes the MongoDB connection
func (m *MongoDB) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return m.client.Disconnect(ctx)
}

// InsertOperation inserts an operation into MongoDB
func (m *MongoDB) InsertOperation(ctx context.Context, op *models.Operation) error {
	op.CreatedAt = time.Now()
	_, err := m.operations.InsertOne(ctx, op)
	return err
}

// InsertOperations inserts multiple operations into MongoDB
// Uses upsert to prevent duplicates based on unique index
func (m *MongoDB) InsertOperations(ctx context.Context, ops []*models.Operation) error {
	if len(ops) == 0 {
		return nil
	}

	now := time.Now()
	for _, op := range ops {
		op.CreatedAt = now

		// Use upsert to prevent duplicates
		filter := bson.M{
			"block_num": op.BlockNum,
			"trx_id":    op.TrxID,
			"op_in_trx": op.OpInTrx,
			"account":   op.Account,
		}

		update := bson.M{
			"$set": op,
		}

		opts := options.Update().SetUpsert(true)
		_, err := m.operations.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			return fmt.Errorf("failed to upsert operation: %w", err)
		}
	}

	return nil
}

// GetOperations retrieves operations with pagination
func (m *MongoDB) GetOperations(ctx context.Context, account string, opType string, page, pageSize int) (*models.OperationResponse, error) {
	filter := bson.M{}
	if account != "" {
		filter["account"] = account
	}
	if opType != "" {
		filter["op_type"] = opType
	}

	// Count total
	total, err := m.operations.CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to count operations: %w", err)
	}

	// Calculate skip
	skip := int64((page - 1) * pageSize)

	// Find operations
	opts := options.Find().
		SetSort(bson.D{{Key: "block_num", Value: -1}, {Key: "timestamp", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(pageSize))

	cursor, err := m.operations.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find operations: %w", err)
	}
	defer cursor.Close(ctx)

	var operations []models.Operation
	if err := cursor.All(ctx, &operations); err != nil {
		return nil, fmt.Errorf("failed to decode operations: %w", err)
	}

	hasMore := skip+int64(len(operations)) < total

	return &models.OperationResponse{
		Operations: operations,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		HasMore:    hasMore,
	}, nil
}

// GetSyncState retrieves the current sync state
func (m *MongoDB) GetSyncState(ctx context.Context) (*models.SyncState, error) {
	var state models.SyncState
	err := m.syncState.FindOne(ctx, bson.M{}).Decode(&state)
	if err == mongo.ErrNoDocuments {
		// Return default state if not found
		return &models.SyncState{
			LastBlock:             0,
			LastIrreversibleBlock: 0,
			UpdatedAt:             time.Now(),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get sync state: %w", err)
	}
	return &state, nil
}

// UpdateSyncState updates the sync state
func (m *MongoDB) UpdateSyncState(ctx context.Context, lastBlock, lastIrreversibleBlock int64) error {
	state := models.SyncState{
		LastBlock:             lastBlock,
		LastIrreversibleBlock: lastIrreversibleBlock,
		UpdatedAt:             time.Now(),
	}

	opts := options.Update().SetUpsert(true)
	filter := bson.M{}
	update := bson.M{"$set": state}

	_, err := m.syncState.UpdateOne(ctx, filter, update, opts)
	return err
}

// GetTrackedAccounts returns list of unique tracked accounts
func (m *MongoDB) GetTrackedAccounts(ctx context.Context) ([]string, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$account"},
		}}},
		{{Key: "$sort", Value: bson.D{
			{Key: "_id", Value: 1},
		}}},
	}

	cursor, err := m.operations.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate accounts: %w", err)
	}
	defer cursor.Close(ctx)

	var results []struct {
		ID string `bson:"_id"`
	}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode accounts: %w", err)
	}

	accounts := make([]string, len(results))
	for i, result := range results {
		accounts[i] = result.ID
	}

	return accounts, nil
}

// CreateIndexes creates necessary indexes for better query performance
func (m *MongoDB) CreateIndexes(ctx context.Context) error {
	// Unique index to prevent duplicate operations
	// An operation is uniquely identified by block_num + trx_id + op_in_trx + account
	uniqueIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "block_num", Value: 1},
			{Key: "trx_id", Value: 1},
			{Key: "op_in_trx", Value: 1},
			{Key: "account", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	// Index on account and block_num for faster queries
	accountIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "account", Value: 1},
			{Key: "block_num", Value: -1},
		},
	}

	// Index on op_type for filtering
	opTypeIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "op_type", Value: 1}},
	}

	// Index on timestamp for sorting
	timestampIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "timestamp", Value: -1}},
	}

	_, err := m.operations.Indexes().CreateMany(ctx, []mongo.IndexModel{
		uniqueIndex,
		accountIndex,
		opTypeIndex,
		timestampIndex,
	})
	return err
}
