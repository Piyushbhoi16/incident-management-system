package repositories

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"ims/internal/domain"
)

type RawSignalRepository interface {
	Store(ctx context.Context, signal domain.Signal, workItemID *string) error
	EnsureIndexes(ctx context.Context) error
}

type MongoRawSignalRepository struct {
	collection *mongo.Collection
}

type rawSignalDocument struct {
	ComponentID string          `bson:"component_id"`
	Severity    domain.Severity `bson:"severity"`
	Message     string          `bson:"message"`
	Timestamp   time.Time       `bson:"timestamp"`
	WorkItemID  *string         `bson:"work_item_id"`
}

func NewMongoRawSignalRepository(collection *mongo.Collection) *MongoRawSignalRepository {
	return &MongoRawSignalRepository{collection: collection}
}

func (r *MongoRawSignalRepository) Store(ctx context.Context, signal domain.Signal, workItemID *string) error {
	doc := rawSignalDocument{
		ComponentID: signal.ComponentID,
		Severity:    signal.Severity,
		Message:     signal.Message,
		Timestamp:   signal.Timestamp,
		WorkItemID:  workItemID,
	}

	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("insert raw signal: %w", err)
	}

	return nil
}

func (r *MongoRawSignalRepository) EnsureIndexes(ctx context.Context) error {
	models := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "component_id", Value: 1}},
			Options: options.Index().SetName("idx_raw_signals_component_id"),
		},
		{
			Keys:    bson.D{{Key: "timestamp", Value: -1}},
			Options: options.Index().SetName("idx_raw_signals_timestamp"),
		},
		{
			Keys: bson.D{
				{Key: "component_id", Value: 1},
				{Key: "timestamp", Value: -1},
			},
			Options: options.Index().SetName("idx_raw_signals_component_id_timestamp"),
		},
	}

	if _, err := r.collection.Indexes().CreateMany(ctx, models); err != nil {
		return fmt.Errorf("create raw signal indexes: %w", err)
	}

	return nil
}
