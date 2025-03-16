package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"log/slog"
	"webPageAnalyzerGO/internal/analyzer"
	"webPageAnalyzerGO/internal/config"
	"webPageAnalyzerGO/internal/middleware"
	"webPageAnalyzerGO/internal/models"
	"webPageAnalyzerGO/internal/repository"
)

// Server represents the HTTP server
type Server struct {
	router     *gin.Engine
	httpServer *http.Server
	repo       repository.Repository
	analyzer   *analyzer.Analyzer
	auth       *middleware.KeycloakAuth
	logger     *slog.Logger
	config     *config.Config
}

// NewServer creates a new HTTP server
func NewServer(cfg *config.Config, repo repository.Repository, logger *slog.Logger) *Server {
	// Set Gin mode
	if gin.Mode() == gin.DebugMode {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add recovery middleware
	router.Use(gin.Recovery())

	// Add CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Create Keycloak auth middleware
	auth := middleware.NewKeycloakAuth(&cfg.Keycloak, logger)

	// Create the server
	s := &Server{
		router: router,
		httpServer: &http.Server{
			Addr:         ":" + cfg.Server.Port,
			Handler:      router,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
		},
		repo:     repo,
		analyzer: analyzer.New(cfg.Analyzer, logger),
		auth:     auth,
		logger:   logger,
		config:   cfg,
	}

	// Register routes
	s.registerRoutes()

	return s
}

// Start starts the HTTP server
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// registerRoutes sets up all the routes for the server
func (s *Server) registerRoutes() {
	// Health check
	s.router.GET("/health", s.healthHandler)

	// Public API routes
	public := s.router.Group("/api")
	{
		// Analyze URL (public endpoint for demo purposes)
		public.POST("/analyze", s.analyzeURLHandler)
	}

	// Protected API routes
	protected := s.router.Group("/api")
	protected.Use(s.auth.Authenticate())
	{
		// Get analysis by ID
		protected.GET("/analysis/:id", s.getAnalysisHandler)

		// Get deep analysis
		protected.GET("/analysis/:id/deep", s.deepAnalysisHandler)

		// Get recent analyses
		protected.GET("/analyses", s.getRecentAnalysesHandler)

		// Get current user's analyses
		protected.GET("/user/analyses", s.getUserAnalysesHandler)
	}

	// Admin-only routes
	admin := s.router.Group("/api/admin")
	admin.Use(s.auth.Authenticate(), s.auth.RequireRoles("admin"))
	{
		// Admin endpoints would go here
		admin.GET("/stats", s.getStatsHandler)
	}
}

// healthHandler handles health check requests
func (s *Server) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// analyzeURLHandler handles requests to analyze a URL
func (s *Server) analyzeURLHandler(c *gin.Context) {
	// Parse request
	var req struct {
		URL string `json:"url" binding:"required,url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status_code": http.StatusBadRequest,
			"message":     "Invalid request",
			"error":       err.Error(),
		})
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), s.config.Analyzer.RequestTimeout)
	defer cancel()

	// Analyze URL
	s.logger.Info("Analyzing URL", "url", req.URL)
	result, err := s.analyzer.AnalyzeURL(ctx, req.URL)
	if err != nil {
		s.logger.Error("Failed to analyze URL", "url", req.URL, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status_code": http.StatusBadRequest,
			"message":     fmt.Sprintf("Failed to analyze URL: %s", req.URL),
			"error":       err.Error(),
		})
		return
	}

	// If user is authenticated, associate analysis with user
	if userInfo, exists := c.Get("userInfo"); exists {
		if ui, ok := userInfo.(*middleware.UserInfo); ok {
			result.UserID = ui.Sub
		}
	}

	// Save analysis to database
	if err := s.repo.SaveAnalysis(ctx, result); err != nil {
		s.logger.Error("Failed to save analysis", "error", err)
		// Continue anyway, just log the error
	}

	// Return result
	c.JSON(http.StatusOK, result)
}

// getAnalysisHandler handles requests to get an analysis by ID
func (s *Server) getAnalysisHandler(c *gin.Context) {
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
	result, err := s.repo.GetAnalysis(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get analysis", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status_code": http.StatusInternalServerError,
			"message":     "Failed to get analysis",
			"error":       err.Error(),
		})
		return
	}

	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status_code": http.StatusNotFound,
			"message":     "Analysis not found",
		})
		return
	}

	// Get authenticated user info
	userInfo, exists := c.Get("userInfo")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status_code": http.StatusUnauthorized,
			"message":     "Unauthorized",
		})
		return
	}

	// Check if the analysis belongs to the user or if user is admin
	ui := userInfo.(*middleware.UserInfo)
	isAdmin := false
	for _, role := range ui.RealmAccess.Roles {
		if role == "admin" {
			isAdmin = true
			break
		}
	}

	if !isAdmin && result.UserID != "" && result.UserID != ui.Sub {
		c.JSON(http.StatusForbidden, gin.H{
			"status_code": http.StatusForbidden,
			"message":     "You don't have permission to access this analysis",
		})
		return
	}

	// Return result
	c.JSON(http.StatusOK, result)
}

// getRecentAnalysesHandler handles requests to get recent analyses
func (s *Server) getRecentAnalysesHandler(c *gin.Context) {
	// Default limit to 10
	limit := 10

	// Try to get limit from query parameter
	if limitParam := c.Query("limit"); limitParam != "" {
		if n, err := fmt.Sscanf(limitParam, "%d", &limit); err != nil || n != 1 {
			// Invalid limit, use default
			limit = 10
		}
	}

	// Cap limit to reasonable value
	if limit > 100 {
		limit = 100
	}

	// Get authenticated user info
	userInfo, exists := c.Get("userInfo")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status_code": http.StatusUnauthorized,
			"message":     "Unauthorized",
		})
		return
	}

	// Check if user is admin
	ui := userInfo.(*middleware.UserInfo)
	isAdmin := false
	for _, role := range ui.RealmAccess.Roles {
		if role == "admin" {
			isAdmin = true
			//break
		}
	}

	// Get recent analyses from database
	ctx := c.Request.Context()
	var results []*models.AnalysisResult
	var err error

	if isAdmin {
		// Admins can see all analyses
		results, err = s.repo.GetRecentAnalyses(ctx, limit)
	} else {
		// Regular users can only see their own analyses
		//results, err = s.repo.GetUserAnalyses(ctx, ui.Sub, limit)
		results, err = s.repo.GetRecentAnalyses(ctx, limit)
	}

	if err != nil {
		s.logger.Error("Failed to get recent analyses", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status_code": http.StatusInternalServerError,
			"message":     "Failed to get recent analyses",
			"error":       err.Error(),
		})
		return
	}

	// Return results
	c.JSON(http.StatusOK, gin.H{
		"count":    len(results),
		"analyses": results,
	})
}

// getUserAnalysesHandler handles requests to get the current user's analyses
func (s *Server) getUserAnalysesHandler(c *gin.Context) {
	// Default limit to 10
	limit := 10

	// Try to get limit from query parameter
	if limitParam := c.Query("limit"); limitParam != "" {
		if n, err := fmt.Sscanf(limitParam, "%d", &limit); err != nil || n != 1 {
			// Invalid limit, use default
			limit = 10
		}
	}

	// Cap limit to reasonable value
	if limit > 100 {
		limit = 100
	}

	// Get authenticated user info
	userInfo, exists := c.Get("userInfo")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status_code": http.StatusUnauthorized,
			"message":     "Unauthorized",
		})
		return
	}

	ui := userInfo.(*middleware.UserInfo)

	// Get user analyses from database
	ctx := c.Request.Context()
	results, err := s.repo.GetUserAnalyses(ctx, ui.Sub, limit)
	if err != nil {
		s.logger.Error("Failed to get user analyses", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status_code": http.StatusInternalServerError,
			"message":     "Failed to get user analyses",
			"error":       err.Error(),
		})
		return
	}

	// Return results
	c.JSON(http.StatusOK, gin.H{
		"count":    len(results),
		"analyses": results,
	})
}

// getStatsHandler handles requests to get admin stats
func (s *Server) getStatsHandler(c *gin.Context) {
	// Get admin stats
	ctx := c.Request.Context()
	stats, err := s.repo.GetStats(ctx)
	if err != nil {
		s.logger.Error("Failed to get stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status_code": http.StatusInternalServerError,
			"message":     "Failed to get stats",
			"error":       err.Error(),
		})
		return
	}

	// Return stats
	c.JSON(http.StatusOK, stats)
}
