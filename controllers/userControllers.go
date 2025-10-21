package controllers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/auth"
	"github.com/gin-gonic/gin"
)

// --- Helper Structs ---

type LoginRequest struct {
	IDToken string `json:"idToken" binding:"required"`
}

type UserData struct {
	UID         string    `firestore:"uid"`
	Email       string    `firestore:"email,omitempty"`
	DisplayName string    `firestore:"displayName,omitempty"`
	PhoneNumber string    `firestore:"phoneNumber,omitempty"`
	CreatedAt   time.Time `firestore:"createdAt"`
	// Add other user fields as needed (e.g., photoUrl, roles)
}

// --- Public Handlers ---

// RegisterUser handles the creation of a new user document after Firebase authentication.
// This is typically called after the client successfully completes Google Sign-In or Phone/OTP flow.
func RegisterUser(c *gin.Context, firestoreClient *firestore.Client, authClient *auth.Client) {
	var req struct {
		IDToken string `json:"idToken" binding:"required"`
		// Optional fields from the client if registration logic is complex
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: IDToken required"})
		return
	}

	ctx := context.Background()

	// 1. Verify the Firebase ID Token
	token, err := authClient.VerifyIDToken(ctx, req.IDToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired Firebase ID Token"})
		return
	}

	uid := token.UID

	// 2. Check if user document already exists in Firestore
	userRef := firestoreClient.Collection("users").Doc(uid)
	doc, err := userRef.Get(ctx)
	if err != nil && !doc.Exists() {
		// User document does not exist, create it
		metadata := UserData{
			UID:         uid,
			Email:       token.Claims["email"].(string),
			DisplayName: token.Claims["name"].(string),
			CreatedAt:   time.Now(),
		}

		_, setErr := userRef.Set(ctx, metadata)
		if setErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create user document: %v", setErr)})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": "User registered and logged in successfully", "uid": uid})
		return
	} else if err != nil {
		// Some other Firestore error occurred
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Firestore error: %v", err)})
		return
	}

	// User document already exists (act as a login)
	c.JSON(http.StatusOK, gin.H{"message": "User already registered and logged in successfully", "uid": uid})
}

// LoginUser verifies the ID Token provided by the client after successful Firebase Auth flow.
func LoginUser(c *gin.Context, authClient *auth.Client) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: IDToken required"})
		return
	}

	ctx := context.Background()

	// 1. Verify the Firebase ID Token (which acts as the session token)
	token, err := authClient.VerifyIDToken(ctx, req.IDToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired Firebase ID Token"})
		return
	}

	// 2. Successful verification means the user is authenticated.
	// The ID token itself is the JWT, which is handled on the protected routes via middleware.
	c.JSON(http.StatusOK, gin.H{
		"message":   "Authentication successful",
		"uid":       token.UID,
		"token":     req.IDToken, // Echo the token back (optional)
		"expiresIn": 3600,        // JWT standard expiry for ID Token is 1 hour
	})
}

// ListUsers is a protected route example.
func ListUsers(c *gin.Context, firestoreClient *firestore.Client) {
	// The middleware ensures the user is authenticated and the UID is in the context.
	uid, _ := c.Get("uid")

	// This is just a placeholder; you should typically fetch user profiles here.
	c.JSON(http.StatusOK, gin.H{
		"message":          "Access granted to protected resource",
		"authenticated_as": uid,
		"detail":           "Listing users not implemented, but auth works!",
	})
}
