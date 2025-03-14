package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AnalysisRequest represents the request to analyze a URL
type AnalysisRequest struct {
	URL string `json:"url" binding:"required,url"`
}

// LinkStatus represents the status of a link
type LinkStatus struct {
	Count        int `json:"count"`
	Inaccessible int `json:"inaccessible"`
}

// HeadingCount represents the count of headings by level
type HeadingCount struct {
	H1 int `json:"h1"`
	H2 int `json:"h2"`
	H3 int `json:"h3"`
	H4 int `json:"h4"`
	H5 int `json:"h5"`
	H6 int `json:"h6"`
}

// AnalysisResult represents the result of a webpage analysis
type AnalysisResult struct {
	ID            primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	URL           string             `json:"url" bson:"url"`
	HTMLVersion   string             `json:"html_version" bson:"html_version"`
	Title         string             `json:"title" bson:"title"`
	Headings      HeadingCount       `json:"headings" bson:"headings"`
	InternalLinks LinkStatus         `json:"internal_links" bson:"internal_links"`
	ExternalLinks LinkStatus         `json:"external_links" bson:"external_links"`
	HasLoginForm  bool               `json:"has_login_form" bson:"has_login_form"`
	UserID        string             `json:"user_id,omitempty" bson:"user_id,omitempty"`
	CreatedAt     time.Time          `json:"created_at" bson:"created_at"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Error      string `json:"error,omitempty"`
}

// Stats represents application statistics
type Stats struct {
	TotalAnalyses      int       `json:"total_analyses" bson:"total_analyses"`
	UniqueURLs         int       `json:"unique_urls" bson:"unique_urls"`
	RegisteredUsers    int       `json:"registered_users" bson:"registered_users"`
	AnalysesLast24h    int       `json:"analyses_last_24h" bson:"analyses_last_24h"`
	AnalysesLast7d     int       `json:"analyses_last_7d" bson:"analyses_last_7d"`
	AnalysesLast30d    int       `json:"analyses_last_30d" bson:"analyses_last_30d"`
	MostAnalyzedDomain string    `json:"most_analyzed_domain" bson:"most_analyzed_domain"`
	LastUpdated        time.Time `json:"last_updated" bson:"last_updated"`
}
