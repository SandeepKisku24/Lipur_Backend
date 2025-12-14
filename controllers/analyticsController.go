package controllers

import (
	"context"
	"log"
	"net/http"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
)

// RecordListeningSession stores exactly how long a user listened to a song
func RecordListeningSession(c *gin.Context, firestoreClient *firestore.Client) {
	var request struct {
		UserId           string  `json:"userId"` // <--- NEW FIELD
		SongId           string  `json:"songId"`
		DurationListened float64 `json:"durationListened"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Use the ID sent from frontend, or default to "anonymous"
	targetUserId := request.UserId
	if targetUserId == "" {
		targetUserId = "anonymous"
	}

	// Validation: Ignore noise (e.g., listened for < 1 second)
	if request.DurationListened < 1 {
		c.JSON(http.StatusOK, gin.H{"message": "Ignored short listen"})
		return
	}

	ctx := context.Background()

	// 1. Log to User's History (Granular Data)
	// Keep this! It's crucial for recommendation algorithms later.
	historyEntry := map[string]interface{}{
		"songId":           request.SongId,
		"durationListened": request.DurationListened,
		"timestamp":        time.Now(),
	}
	_, _, err := firestoreClient.Collection("users").Doc(targetUserId).Collection("listening_history").Add(ctx, historyEntry)
	if err != nil {
		log.Printf("Failed to log history: %v", err)
	}

	// 2. Increment Global Song Stats (Aggregated Data)
	// We update the song document to add this session's time to the global total.
	songRef := firestoreClient.Collection("songs").Doc(request.SongId)

	updates := []firestore.Update{
		// Always increment the total time played (even if it's just 5 seconds)
		{Path: "totalPlayTime", Value: firestore.Increment(request.DurationListened)},
	}

	// Only increment "playCount" if it counts as a "valid view" (e.g. > 30 seconds)
	if request.DurationListened > 30 {
		updates = append(updates, firestore.Update{Path: "playCount", Value: firestore.Increment(1)})
	}

	_, err = songRef.Update(ctx, updates)
	if err != nil {
		log.Printf("Failed to update song stats: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session recorded"})
}

// UpdateSongMetadata allows the frontend to patch missing data (like Duration)
// This solves your "Old music has zero duration" problem.
func UpdateSongMetadata(c *gin.Context, firestoreClient *firestore.Client) {
	// Security: You might want to restrict this to Admins OR verify the duration is reasonable.
	// For now, we allow authenticated users to "help" fix the DB.

	songId := c.Param("id")
	var request struct {
		Duration float64 `json:"duration"` // The real duration from React Native
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	ctx := context.Background()
	songRef := firestoreClient.Collection("songs").Doc(songId)

	// Check if it really needs updating (Optimization)
	doc, err := songRef.Get(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Song not found"})
		return
	}

	// Only update if the current duration is 0 or missing
	currentDuration := 0.0
	if d, ok := doc.Data()["duration"].(float64); ok {
		currentDuration = d
	}

	// If DB has 0, but Frontend sent a real number, update it.
	if currentDuration == 0 && request.Duration > 0 {
		_, err = songRef.Update(ctx, []firestore.Update{
			{Path: "duration", Value: request.Duration},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update"})
			return
		}
		log.Printf("Crowdsource Fix: Updated duration for song %s to %.2f", songId, request.Duration)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Metadata updated"})
}

func GetListeningHistory(c *gin.Context, firestoreClient *firestore.Client) {
	// Get userId from Query Param (e.g. ?userId=xyz)
	userId := c.Query("userId")
	if userId == "" {
		userId = "anonymous"
	}

	ctx := context.Background()

	// Fetch last 50 entries (we fetch more than needed to handle duplicates)
	docs, err := firestoreClient.Collection("users").Doc(userId).Collection("listening_history").
		OrderBy("timestamp", firestore.Desc).
		Limit(50).
		Documents(ctx).
		GetAll()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch history"})
		return
	}

	var history []map[string]interface{}
	for _, doc := range docs {
		data := doc.Data()
		// Convert timestamp to unix for easier frontend handling
		if ts, ok := data["timestamp"].(time.Time); ok {
			data["timestamp"] = ts.Unix()
		}
		history = append(history, data)
	}

	c.JSON(http.StatusOK, gin.H{"history": history})
}
