package middleware

import (
	"context"
	"net/http"
	"strings"

	"firebase.google.com/go/auth"
	"github.com/gin-gonic/gin"
)

// AuthMiddleware creates a middleware that verifies the Firebase ID Token (JWT).
func AuthMiddleware(authClient *auth.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Extract the Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// 2. Check for "Bearer " prefix and extract the token
		idTokenParts := strings.SplitN(authHeader, " ", 2)
		if len(idTokenParts) != 2 || strings.ToLower(idTokenParts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization header format. Must be Bearer <token>"})
			c.Abort()
			return
		}
		idToken := idTokenParts[1]

		ctx := context.Background()

		// 3. Verify the Firebase ID Token (This verifies the JWT signature and expiration)
		token, err := authClient.VerifyIDToken(ctx, idToken)
		if err != nil {
			// Token is invalid (expired, bad signature, etc.)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed: Invalid or expired token"})
			c.Abort()
			return
		}

		// 4. Authentication successful: Inject the UID into the Gin context
		c.Set("uid", token.UID)

		// Continue to the next handler
		c.Next()
	}
}
