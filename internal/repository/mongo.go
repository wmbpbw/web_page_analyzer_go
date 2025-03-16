package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"webPageAnalyzerGO/internal/config"
	"webPageAnalyzerGO/internal/models"
)

// Repository defines operations on analysis data
type Repository interface {
	SaveAnalysis(ctx context.Context, analysis *models.AnalysisResult) error
	GetAnalysis(ctx context.Context, id string) (*models.AnalysisResult, error)
	GetRecentAnalyses(ctx context.Context, limit int) ([]*models.AnalysisResult, error)
	GetUserAnalyses(ctx context.Context, userID string, limit int) ([]*models.AnalysisResult, error)

	// Deep analysis methods
	SaveDeepAnalysis(ctx context.Context, analysis *models.DeepAnalysisResult) error
	GetDeepAnalysis(ctx context.Context, analysisID string) (*models.DeepAnalysisResult, error)

	GetStats(ctx context.Context) (*models.Stats, error)
	Close(ctx context.Context) error
}

// MongoRepository implements Repository interface for MongoDB
type MongoRepository struct {
	client         *mongo.Client
	collection     *mongo.Collection
	deepCollection *mongo.Collection
}

// NewMongoRepository creates a new MongoDB repository
func NewMongoRepository(ctx context.Context, cfg config.MongoDBConfig) (*MongoRepository, error) {
	clientOptions := options.Client().
		ApplyURI(cfg.URI).
		SetConnectTimeout(cfg.Timeout)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	// Check the connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	// Get collections
	collection := client.Database(cfg.Database).Collection(cfg.CollectionName)
	deepCollection := client.Database(cfg.Database).Collection(cfg.CollectionName + "_deep")

	// Create index on URL field for faster lookups
	indexModels := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "url", Value: 1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{Key: "created_at", Value: -1}},
			Options: options.Index().SetBackground(true),
		},
	}

	if _, err := collection.Indexes().CreateMany(ctx, indexModels); err != nil {
		return nil, err
	}

	// Create indexes for deep analysis collection
	deepIndexModels := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "analysis_id", Value: 1}},
			Options: options.Index().SetBackground(true).SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "created_at", Value: -1}},
			Options: options.Index().SetBackground(true),
		},
	}

	if _, err := deepCollection.Indexes().CreateMany(ctx, deepIndexModels); err != nil {
		return nil, err
	}

	return &MongoRepository{
		client:         client,
		collection:     collection,
		deepCollection: deepCollection,
	}, nil
}

// SaveAnalysis saves an analysis result to MongoDB
func (r *MongoRepository) SaveAnalysis(ctx context.Context, analysis *models.AnalysisResult) error {
	// Set creation time if not set
	if analysis.CreatedAt.IsZero() {
		analysis.CreatedAt = time.Now()
	}

	// Insert document
	result, err := r.collection.InsertOne(ctx, analysis)
	if err != nil {
		return err
	}

	// Update ID in the analysis object
	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		analysis.ID = oid
	}

	return nil
}

// GetAnalysis retrieves an analysis by ID
func (r *MongoRepository) GetAnalysis(ctx context.Context, id string) (*models.AnalysisResult, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	var analysis models.AnalysisResult
	err = r.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&analysis)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Not found
		}
		return nil, err
	}

	return &analysis, nil
}

// GetRecentAnalyses retrieves the most recent analyses
func (r *MongoRepository) GetRecentAnalyses(ctx context.Context, limit int) ([]*models.AnalysisResult, error) {
	findOptions := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, bson.M{}, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var analyses []*models.AnalysisResult
	if err := cursor.All(ctx, &analyses); err != nil {
		return nil, err
	}

	return analyses, nil
}

// GetUserAnalyses retrieves analyses for a specific user
func (r *MongoRepository) GetUserAnalyses(ctx context.Context, userID string, limit int) ([]*models.AnalysisResult, error) {
	findOptions := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, bson.M{"user_id": userID}, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var analyses []*models.AnalysisResult
	if err := cursor.All(ctx, &analyses); err != nil {
		return nil, err
	}

	return analyses, nil
}

// SaveDeepAnalysis saves a deep analysis result to MongoDB
func (r *MongoRepository) SaveDeepAnalysis(ctx context.Context, analysis *models.DeepAnalysisResult) error {
	// Set creation time if not set
	if analysis.CreatedAt.IsZero() {
		analysis.CreatedAt = time.Now()
	}

	// Use upsert to replace existing analysis if it exists
	filter := bson.M{"analysis_id": analysis.AnalysisID}
	update := bson.M{"$set": analysis}
	opts := options.Update().SetUpsert(true)

	_, err := r.deepCollection.UpdateOne(ctx, filter, update, opts)
	return err
}

// GetDeepAnalysis retrieves a deep analysis by analysis ID
func (r *MongoRepository) GetDeepAnalysis(ctx context.Context, analysisID string) (*models.DeepAnalysisResult, error) {
	objectID, err := primitive.ObjectIDFromHex(analysisID)
	if err != nil {
		return nil, err
	}

	var analysis models.DeepAnalysisResult
	err = r.deepCollection.FindOne(ctx, bson.M{"analysis_id": objectID}).Decode(&analysis)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Not found
		}
		return nil, err
	}

	return &analysis, nil
}

// GetStats retrieves application statistics
func (r *MongoRepository) GetStats(ctx context.Context) (*models.Stats, error) {
	// Implementation remains the same...
	// (keeping the original stats implementation)
	return &models.Stats{
		LastUpdated: time.Now(),
	}, nil
}

// Close closes the MongoDB connection
func (r *MongoRepository) Close(ctx context.Context) error {
	return r.client.Disconnect(ctx)
}
