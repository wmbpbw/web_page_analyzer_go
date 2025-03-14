package repository

import (
	"context"
	"net/url"
	"strings"
	"time"
	"webPageAnalyzerGO/internal/config"
	"webPageAnalyzerGO/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Repository defines operations on analysis data
type Repository interface {
	SaveAnalysis(ctx context.Context, analysis *models.AnalysisResult) error
	GetAnalysis(ctx context.Context, id string) (*models.AnalysisResult, error)
	GetRecentAnalyses(ctx context.Context, limit int) ([]*models.AnalysisResult, error)
	GetUserAnalyses(ctx context.Context, userID string, limit int) ([]*models.AnalysisResult, error)
	GetStats(ctx context.Context) (*models.Stats, error)
	Close(ctx context.Context) error
}

// MongoRepository implements Repository interface for MongoDB
type MongoRepository struct {
	client     *mongo.Client
	collection *mongo.Collection
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

	// Get collection
	collection := client.Database(cfg.Database).Collection(cfg.CollectionName)

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

	return &MongoRepository{
		client:     client,
		collection: collection,
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

// GetStats retrieves application statistics
func (r *MongoRepository) GetStats(ctx context.Context) (*models.Stats, error) {
	// Calculate total analyses
	totalAnalyses, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	// Calculate unique URLs
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{"_id": "$url"}}},
		{{Key: "$count", Value: "count"}},
	}
	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var uniqueURLsResult []bson.M
	if err := cursor.All(ctx, &uniqueURLsResult); err != nil {
		return nil, err
	}

	var uniqueURLs int
	if len(uniqueURLsResult) > 0 {
		uniqueURLs = int(uniqueURLsResult[0]["count"].(int32))
	}

	// Calculate registered users
	pipeline = mongo.Pipeline{

		{{Key: "$match", Value: bson.M{"user_id": bson.M{"$ne": nil, "_$ne_": ""}}}},
		{{Key: "$group", Value: bson.M{"_id": "$user_id"}}},
		{{Key: "$count", Value: "count"}},
	}
	cursor, err = r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var registeredUsersResult []bson.M
	if err := cursor.All(ctx, &registeredUsersResult); err != nil {
		return nil, err
	}

	var registeredUsers int
	if len(registeredUsersResult) > 0 {
		registeredUsers = int(registeredUsersResult[0]["count"].(int32))
	}

	// Calculate analyses in the last 24 hours
	oneDayAgo := time.Now().Add(-24 * time.Hour)
	analysesLast24h, err := r.collection.CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": oneDayAgo},
	})
	if err != nil {
		return nil, err
	}

	// Calculate analyses in the last 7 days
	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour)
	analysesLast7d, err := r.collection.CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": sevenDaysAgo},
	})
	if err != nil {
		return nil, err
	}

	// Calculate analyses in the last 30 days
	thirtyDaysAgo := time.Now().Add(-30 * 24 * time.Hour)
	analysesLast30d, err := r.collection.CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": thirtyDaysAgo},
	})
	if err != nil {
		return nil, err
	}

	// Find most analyzed domain
	/*pipeline = mongo.Pipeline{
		{{Key: "$project", Value: bson.M{
			"domain": bson.M{
				"$let": bson.M{
					"vars": bson.M{
						"parts": bson.M{
							"$split": bson.M{
								"$replaceAll": bson.M{
									"input":       "$url",
									"find":        "https://",
									"replacement": "",
								},
							},
							"/",
						},
					},
					"in": bson.M{
						"$let": bson.M{
							"vars": bson.M{
								"domain": bson.M{
									"$replaceAll": bson.M{
										"input":       bson.M{"$arrayElemAt": bson.A{"$$parts", 0}},
										"find":        "http://",
										"replacement": "",
									},
								},
							},
							"in": "$$domain",
						},
					},
				},
			},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$domain",
			"count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"count": -1}}},
		{{Key: "$limit", Value: 1}},
	}*/

	cursor, err = r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var mostAnalyzedDomainResult []bson.M
	if err := cursor.All(ctx, &mostAnalyzedDomainResult); err != nil {
		return nil, err
	}

	var mostAnalyzedDomain string
	if len(mostAnalyzedDomainResult) > 0 {
		mostAnalyzedDomain = mostAnalyzedDomainResult[0]["_id"].(string)
	}

	// Return stats
	return &models.Stats{
		TotalAnalyses:      int(totalAnalyses),
		UniqueURLs:         uniqueURLs,
		RegisteredUsers:    registeredUsers,
		AnalysesLast24h:    int(analysesLast24h),
		AnalysesLast7d:     int(analysesLast7d),
		AnalysesLast30d:    int(analysesLast30d),
		MostAnalyzedDomain: mostAnalyzedDomain,
		LastUpdated:        time.Now(),
	}, nil
}

// extractDomain extracts domain from URL
func extractDomain(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(parsedURL.Hostname(), "www.")
}

// Close closes the MongoDB connection
func (r *MongoRepository) Close(ctx context.Context) error {
	return r.client.Disconnect(ctx)
}
