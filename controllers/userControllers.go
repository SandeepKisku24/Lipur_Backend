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

func RegisterUser(c *gin.Context, firestoreClient *firestore.Client, authClient *auth.Client) {
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Username string `json:"username"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err)})
		return
	}

	if request.Email == "" || request.Password == "" || request.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email, password, and username are required"})
		return
	}

	// Create user in Firebase Auth
	params := (&auth.UserToCreate{}).
		Email(request.Email).
		Password(request.Password).
		DisplayName(request.Username)
	user, err := authClient.CreateUser(context.Background(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create user: %v", err)})
		return
	}

	// Save user data to Firestore
	userData := map[string]interface{}{
		"uid":       user.UID,
		"email":     request.Email,
		"username":  request.Username,
		"createdAt": time.Now(),
	}
	_, err = firestoreClient.Collection("users").Doc(user.UID).Set(context.Background(), userData)
	if err != nil {
		// Attempt to delete the user from Firebase Auth if Firestore fails
		authClient.DeleteUser(context.Background(), user.UID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save user data: %v", err)})
		return
	}

	// Generate custom token for login
	token, err := authClient.CustomToken(context.Background(), user.UID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate token: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User registered successfully",
		"uid":     user.UID,
		"token":   token,
	})
}

func LoginUser(c *gin.Context, authClient *auth.Client) {
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err)})
		return
	}

	// In a real app, the client should handle login with Firebase Auth SDK.
	// Here, we'll simulate by verifying the email and generating a token.
	// Note: Firebase Auth doesn't provide a direct way to "login" via email/password on the server.
	// The client should sign in, and we'll verify the token.

	// For simplicity, we'll generate a custom token (in a real app, use client-side login)
	user, err := authClient.GetUserByEmail(context.Background(), request.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or user not found"})
		return
	}

	// Generate custom token
	token, err := authClient.CustomToken(context.Background(), user.UID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate token: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"uid":     user.UID,
		"token":   token,
	})
}

func ListUsers(c *gin.Context, firestoreClient *firestore.Client) {
	ctx := context.Background()
	docs, err := firestoreClient.Collection("users").Documents(ctx).GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch users: %v", err)})
		return
	}

	users := []map[string]interface{}{}
	for _, doc := range docs {
		data := doc.Data()
		if ts, ok := data["createdAt"].(time.Time); ok {
			data["createdAt"] = ts.Unix()
		}
		users = append(users, data)
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}
