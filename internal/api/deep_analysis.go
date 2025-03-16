package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	_ "regexp"
	_ "strings"
	"time"
	"webPageAnalyzerGO/internal/middleware"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	_ "webPageAnalyzerGO/internal/analyzer"
	"webPageAnalyzerGO/internal/models"
)

// deepAnalysisHandler handles requests for deep analysis of a web page
func (s *Server) deepAnalysisHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status_code": http.StatusBadRequest,
			"message":     "Missing analysis ID",
		})
		return
	}

	// Get analysis from database
	ctx := c.Request.Context()
	analysis, err := s.repo.GetAnalysis(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get analysis", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status_code": http.StatusInternalServerError,
			"message":     "Failed to get analysis",
			"error":       err.Error(),
		})
		return
	}

	if analysis == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status_code": http.StatusNotFound,
			"message":     "Analysis not found",
		})
		return
	}

	// Check permissions if the user is not an admin
	/*if !isAdmin(c) && analysis.UserID != getUserID(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"status_code": http.StatusForbidden,
			"message":     "You don't have permission to access this analysis",
		})
		return
	}*/

	// Check if deep analysis exists
	deepAnalysis, err := s.repo.GetDeepAnalysis(ctx, id)
	if err != nil {
		s.logger.Error("Failed to check for deep analysis", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status_code": http.StatusInternalServerError,
			"message":     "Failed to check for deep analysis",
			"error":       err.Error(),
		})
		return
	}

	// If deep analysis exists and is recent (less than 1 hour old), return it
	if deepAnalysis != nil && time.Since(deepAnalysis.CreatedAt) < time.Hour {
		c.JSON(http.StatusOK, deepAnalysis)
		return
	}

	// Perform deep analysis
	deepAnalysisResult, err := s.performDeepAnalysis(ctx, analysis)
	if err != nil {
		s.logger.Error("Failed to perform deep analysis", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status_code": http.StatusInternalServerError,
			"message":     "Failed to perform deep analysis",
			"error":       err.Error(),
		})
		return
	}

	// Save deep analysis to database
	deepAnalysisResult.ID = primitive.NewObjectID()
	deepAnalysisResult.AnalysisID = analysis.ID
	deepAnalysisResult.CreatedAt = time.Now()

	if err := s.repo.SaveDeepAnalysis(ctx, deepAnalysisResult); err != nil {
		s.logger.Error("Failed to save deep analysis", "id", id, "error", err)
		// Continue anyway, just log the error
	}

	// Return result
	c.JSON(http.StatusOK, deepAnalysisResult)
}

// performDeepAnalysis performs a deep analysis of a web page
func (s *Server) performDeepAnalysis(ctx context.Context, analysis *models.AnalysisResult) (*models.DeepAnalysisResult, error) {
	// Create timeout context
	ctxWithTimeout, cancel := context.WithTimeout(ctx, s.config.Analyzer.RequestTimeout*2)
	defer cancel()

	// Re-fetch the page for more detailed analysis
	page, err := s.analyzer.FetchPage(ctxWithTimeout, analysis.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}

	// Create deep analysis result
	result := &models.DeepAnalysisResult{
		URL: analysis.URL,
		Performance: models.PerformanceMetrics{
			LoadTime:     float64(page.LoadTime.Milliseconds()) / 1000,
			ResourceSize: page.Size,
			Requests:     len(page.Resources),
			TTFB:         float64(page.TTFB.Milliseconds()),
		},
		SEO: models.SEOAnalysis{
			MetaTags: models.MetaTags{
				Title:       page.MetaTags.Title,
				Description: page.MetaTags.Description,
				Keywords:    page.MetaTags.Keywords,
				Robots:      page.MetaTags.Robots,
			},
			Images: models.ImageAnalysis{
				Total:      page.Images.Total,
				MissingAlt: page.Images.MissingAlt,
			},
			HeaderStructure: models.HeaderStructure{
				Proper: isProperHeaderStructure(analysis.Headings),
			},
			CanonicalURL: page.MetaTags.Canonical,
		},
		Accessibility: models.AccessibilityAnalysis{
			AriaAttributes:     page.Accessibility.AriaCount,
			ContrastIssues:     page.Accessibility.ContrastIssues,
			KeyboardNavigation: page.Accessibility.KeyboardNavigation,
		},
		Content: models.ContentAnalysis{
			WordCount:        page.Content.WordCount,
			KeywordDensity:   page.Content.KeywordDensity,
			ReadabilityScore: page.Content.ReadabilityScore,
			TextToHTMLRatio:  page.Content.TextToHTMLRatio,
		},
		Security: models.SecurityAnalysis{
			HTTPS:         isHTTPS(analysis.URL),
			CSPHeaders:    page.Security.CSPHeaders,
			XSSProtection: page.Security.XSSProtection,
		},
		Mobile: models.MobileAnalysis{
			Viewport:         page.Mobile.HasViewport,
			ResponsiveDesign: page.Mobile.IsResponsive,
			TouchFriendly:    page.Mobile.IsTouchFriendly,
		},
		Social: models.SocialMediaAnalysis{
			OpenGraph:    page.Social.HasOpenGraph,
			TwitterCards: page.Social.HasTwitterCards,
			SocialLinks:  page.Social.SocialLinksCount,
		},
		Technology: models.TechnologyAnalysis{
			Server:      page.Technology.Server,
			CMS:         page.Technology.CMS,
			Frameworks:  page.Technology.Frameworks,
			Advertising: page.Technology.Advertising,
		},
		Media: models.MediaAnalysis{
			ImagesCount: page.Media.ImagesCount,
			VideoCount:  page.Media.VideoCount,
			AudioCount:  page.Media.AudioCount,
			TotalSize:   page.Media.TotalSize,
		},
		Schema: models.SchemaAnalysis{
			HasSchema:   page.Schema.HasSchema,
			SchemaTypes: page.Schema.SchemaTypes,
			Format:      page.Schema.Format,
		},
		Cookies: models.CookieAnalysis{
			TotalCount: page.Cookies.TotalCount,
			FirstParty: page.Cookies.FirstParty,
			ThirdParty: page.Cookies.ThirdParty,
			HasConsent: page.Cookies.HasConsent,
			MaxAgeDays: page.Cookies.MaxAgeDays,
		},
		Links: models.LinkAnalysis{
			AnchorText:  page.Links.AnchorText,
			NoFollow:    page.Links.NoFollow,
			BrokenLinks: page.Links.BrokenLinks,
			MaxDepth:    page.Links.MaxDepth,
		},
	}

	return result, nil
}

// Helper functions

// isAdmin checks if the user is an admin
func isAdmin(c *gin.Context) bool {
	userInfo, exists := c.Get("userInfo")
	if !exists {
		return false
	}

	ui, ok := userInfo.(*middleware.UserInfo)
	if !ok {
		return false
	}

	for _, role := range ui.RealmAccess.Roles {
		if role == "admin" {
			return true
		}
	}

	return false
}

// getUserID gets the user ID from the context
func getUserID(c *gin.Context) string {
	userInfo, exists := c.Get("userInfo")
	if !exists {
		return ""
	}

	ui, ok := userInfo.(*middleware.UserInfo)
	if !ok {
		return ""
	}

	return ui.Sub
}

// isProperHeaderStructure checks if the header structure is proper
func isProperHeaderStructure(headings models.HeadingCount) bool {
	// A proper header structure typically has:
	// 1. At least one H1
	// 2. No skipping levels (e.g., H1 to H3 without H2)

	if headings.H1 == 0 {
		return false
	}

	// Check for skipped levels
	if headings.H2 == 0 && (headings.H3 > 0 || headings.H4 > 0 || headings.H5 > 0 || headings.H6 > 0) {
		return false
	}

	if headings.H3 == 0 && (headings.H4 > 0 || headings.H5 > 0 || headings.H6 > 0) {
		return false
	}

	if headings.H4 == 0 && (headings.H5 > 0 || headings.H6 > 0) {
		return false
	}

	if headings.H5 == 0 && headings.H6 > 0 {
		return false
	}

	return true
}

// isHTTPS checks if the URL uses HTTPS
func isHTTPS(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	return parsedURL.Scheme == "https"
}
