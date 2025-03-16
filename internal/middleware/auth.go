package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"webPageAnalyzerGO/internal/config"
	/*"log.o/slog"*/)

// KeycloakAuth represents the Keycloak authentication middleware
type KeycloakAuth struct {
	keycloakConfig *config.KeycloakConfig
	logger         *slog.Logger
	jwksCache      map[string]interface{}
	cacheTime      time.Time
}

// UserInfo contains the user information from the JWT token
type UserInfo struct {
	Sub               string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
	RealmAccess       struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

// NewKeycloakAuth creates a new Keycloak authentication middleware
func NewKeycloakAuth(keycloakConfig *config.KeycloakConfig, logger *slog.Logger) *KeycloakAuth {
	return &KeycloakAuth{
		keycloakConfig: keycloakConfig,
		logger:         logger,
		jwksCache:      make(map[string]interface{}),
	}
}

// Authenticate is a middleware to authenticate users with Keycloak
func (k *KeycloakAuth) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := extractToken(c.Request)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status_code": http.StatusUnauthorized,
				"message":     "Unauthorized",
				"error":       "Invalid or missing token",
			})
			c.Abort()
			return
		}

		// Verify token
		userInfo, err := k.verifyToken(c.Request.Context(), token)
		if err != nil {
			k.logger.Error("Failed to verify token", "error", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"status_code": http.StatusUnauthorized,
				"message":     "Unauthorized",
				"error":       "Invalid token",
			})
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("userInfo", userInfo)
		c.Next()
	}
}

// RequireRoles is a middleware to check if the user has the required roles
func (k *KeycloakAuth) RequireRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user info from context
		userInfo, exists := c.Get("userInfo")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status_code": http.StatusUnauthorized,
				"message":     "Unauthorized",
				"error":       "User not authenticated",
			})
			c.Abort()
			return
		}

		// Check roles
		user := userInfo.(*UserInfo)
		if !hasRequiredRole(user, roles) {
			c.JSON(http.StatusForbidden, gin.H{
				"status_code": http.StatusForbidden,
				"message":     "Forbidden",
				"error":       "Insufficient permissions",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// extractToken extracts the bearer token from the Authorization header
func extractToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("authorization header is missing")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", errors.New("invalid authorization header format")
	}

	return parts[1], nil
}

// verifyToken verifies the JWT token with Keycloak
func (k *KeycloakAuth) verifyToken(ctx context.Context, token string) (*UserInfo, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	// Create request to userinfo endpoint
	userinfoURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/userinfo",
		k.keycloakConfig.URL,
		k.keycloakConfig.Realm)

	req, err := http.NewRequestWithContext(ctx, "GET", userinfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add token to request
	req.Header.Set("Authorization", "Bearer "+token)

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get userinfo: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// Try with the fallback URL if the main URL fails
		if fallbackURL := k.keycloakConfig.FallbackURL; fallbackURL != "" && fallbackURL != k.keycloakConfig.URL {
			k.logger.Info("Using fallback URL for token verification",
				"main", k.keycloakConfig.URL,
				"fallback", fallbackURL)

			// Create a new request with the fallback URL
			fallbackUserinfoURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/userinfo",
				fallbackURL,
				k.keycloakConfig.Realm)

			fallbackReq, err := http.NewRequestWithContext(ctx, "GET", fallbackUserinfoURL, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create fallback request: %w", err)
			}

			// Add token to request
			fallbackReq.Header.Set("Authorization", "Bearer "+token)

			// Send fallback request
			fallbackResp, err := client.Do(fallbackReq)
			if err != nil {
				return nil, fmt.Errorf("failed to get userinfo with fallback URL: %w", err)
			}
			defer fallbackResp.Body.Close()

			if fallbackResp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("invalid token or userinfo request failed: %d", fallbackResp.StatusCode)
			}

			// Parse response from fallback
			var userInfo UserInfo
			if err := json.NewDecoder(fallbackResp.Body).Decode(&userInfo); err != nil {
				return nil, fmt.Errorf("failed to parse userinfo response from fallback: %w", err)
			}

			return &userInfo, nil
		}

		return nil, fmt.Errorf("invalid token or userinfo request failed: %d", resp.StatusCode)
	}

	// Parse response
	var userInfo UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse userinfo response: %w", err)
	}

	return &userInfo, nil
}

// hasRequiredRole checks if the user has any of the required roles
func hasRequiredRole(user *UserInfo, requiredRoles []string) bool {
	if len(requiredRoles) == 0 {
		return true
	}

	userRoles := user.RealmAccess.Roles
	for _, required := range requiredRoles {
		for _, role := range userRoles {
			if role == required {
				return true
			}
		}
	}

	return false
}
