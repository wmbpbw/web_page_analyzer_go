package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DeepAnalysisResult represents comprehensive analysis of a webpage
type DeepAnalysisResult struct {
	ID         primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	AnalysisID primitive.ObjectID `json:"analysis_id" bson:"analysis_id"`
	URL        string             `json:"url" bson:"url"`
	CreatedAt  time.Time          `json:"created_at" bson:"created_at"`

	// 1. Performance metrics
	Performance PerformanceMetrics `json:"performance" bson:"performance"`

	// 2. SEO Analysis
	SEO SEOAnalysis `json:"seo" bson:"seo"`

	// 3. Accessibility
	Accessibility AccessibilityAnalysis `json:"accessibility" bson:"accessibility"`

	// 4. Content Analysis
	Content ContentAnalysis `json:"content" bson:"content"`

	// 5. Security
	Security SecurityAnalysis `json:"security" bson:"security"`

	// 6. Mobile Friendliness
	Mobile MobileAnalysis `json:"mobile" bson:"mobile"`

	// 7. Social Media
	Social SocialMediaAnalysis `json:"social" bson:"social"`

	// 8. Technology Stack
	Technology TechnologyAnalysis `json:"technology" bson:"technology"`

	// 9. Media Analysis
	Media MediaAnalysis `json:"media" bson:"media"`

	// 10. Schema.org Markup
	Schema SchemaAnalysis `json:"schema" bson:"schema"`

	// 11. Cookie Usage
	Cookies CookieAnalysis `json:"cookies" bson:"cookies"`

	// 12. Link Analysis
	Links LinkAnalysis `json:"links" bson:"links"`
}

// PerformanceMetrics represents performance metrics of a webpage
type PerformanceMetrics struct {
	LoadTime     float64 `json:"loadTime" bson:"load_time"`         // in seconds
	ResourceSize int64   `json:"resourceSize" bson:"resource_size"` // in bytes
	Requests     int     `json:"requests" bson:"requests"`
	TTFB         float64 `json:"ttfb" bson:"ttfb"` // Time to First Byte in milliseconds
}

// SEOAnalysis represents SEO-related information
type SEOAnalysis struct {
	MetaTags        MetaTags        `json:"metaTags" bson:"meta_tags"`
	Images          ImageAnalysis   `json:"images" bson:"images"`
	HeaderStructure HeaderStructure `json:"headerStructure" bson:"header_structure"`
	CanonicalURL    string          `json:"canonicalUrl" bson:"canonical_url"`
}

// MetaTags represents meta tags in the HTML document
type MetaTags struct {
	Title       string `json:"title" bson:"title"`
	Description string `json:"description" bson:"description"`
	Keywords    string `json:"keywords" bson:"keywords"`
	Robots      string `json:"robots" bson:"robots"`
}

// ImageAnalysis represents image-related information
type ImageAnalysis struct {
	Total      int `json:"total" bson:"total"`
	MissingAlt int `json:"missingAlt" bson:"missing_alt"`
}

// HeaderStructure represents header structure information
type HeaderStructure struct {
	Proper bool `json:"proper" bson:"proper"`
}

// AccessibilityAnalysis represents accessibility-related information
type AccessibilityAnalysis struct {
	AriaAttributes     int  `json:"ariaAttributes" bson:"aria_attributes"`
	ContrastIssues     int  `json:"contrastIssues" bson:"contrast_issues"`
	KeyboardNavigation bool `json:"keyboardNavigation" bson:"keyboard_navigation"`
}

// ContentAnalysis represents content-related metrics
type ContentAnalysis struct {
	WordCount        int                `json:"wordCount" bson:"word_count"`
	KeywordDensity   map[string]float64 `json:"keywordDensity" bson:"keyword_density"`
	ReadabilityScore float64            `json:"readabilityScore" bson:"readability_score"`
	TextToHTMLRatio  float64            `json:"textToHtmlRatio" bson:"text_to_html_ratio"`
}

// SecurityAnalysis represents security-related information
type SecurityAnalysis struct {
	HTTPS         bool `json:"https" bson:"https"`
	CSPHeaders    bool `json:"cspHeaders" bson:"csp_headers"`
	XSSProtection bool `json:"xssProtection" bson:"xss_protection"`
}

// MobileAnalysis represents mobile-friendliness metrics
type MobileAnalysis struct {
	Viewport         bool `json:"viewport" bson:"viewport"`
	ResponsiveDesign bool `json:"responsiveDesign" bson:"responsive_design"`
	TouchFriendly    bool `json:"touchFriendly" bson:"touch_friendly"`
}

// SocialMediaAnalysis represents social media integration metrics
type SocialMediaAnalysis struct {
	OpenGraph    bool `json:"openGraph" bson:"open_graph"`
	TwitterCards bool `json:"twitterCards" bson:"twitter_cards"`
	SocialLinks  int  `json:"socialLinks" bson:"social_links"`
}

// TechnologyAnalysis represents technology stack information
type TechnologyAnalysis struct {
	Server      string   `json:"server" bson:"server"`
	CMS         string   `json:"cms" bson:"cms"`
	Frameworks  []string `json:"frameworks" bson:"frameworks"`
	Advertising []string `json:"advertising" bson:"advertising"`
}

// MediaAnalysis represents media usage metrics
type MediaAnalysis struct {
	ImagesCount int   `json:"imagesCount" bson:"images_count"`
	VideoCount  int   `json:"videoCount" bson:"video_count"`
	AudioCount  int   `json:"audioCount" bson:"audio_count"`
	TotalSize   int64 `json:"totalSize" bson:"total_size"` // in bytes
}

// SchemaAnalysis represents schema.org markup information
type SchemaAnalysis struct {
	HasSchema   bool     `json:"hasSchema" bson:"has_schema"`
	SchemaTypes []string `json:"schemaTypes" bson:"schema_types"`
	Format      string   `json:"format" bson:"format"` // JSON-LD, Microdata, etc.
}

// CookieAnalysis represents cookie usage metrics
type CookieAnalysis struct {
	TotalCount int  `json:"totalCount" bson:"total_count"`
	FirstParty int  `json:"firstParty" bson:"first_party"`
	ThirdParty int  `json:"thirdParty" bson:"third_party"`
	HasConsent bool `json:"hasConsent" bson:"has_consent"`
	MaxAgeDays int  `json:"maxAgeDays" bson:"max_age_days"`
}

// LinkAnalysis represents link-related metrics
type LinkAnalysis struct {
	AnchorText  map[string]int `json:"anchorText" bson:"anchor_text"`
	NoFollow    int            `json:"noFollow" bson:"no_follow"`
	BrokenLinks int            `json:"brokenLinks" bson:"broken_links"`
	MaxDepth    int            `json:"maxDepth" bson:"max_depth"`
}
