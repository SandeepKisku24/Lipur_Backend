package controllers

import (
	"context"
	"fmt"
	"lipur_backend/services"
	"net/http"
	"strconv"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
)

// ToggleLikeSong updates total counts and populates the user's specific Liked section
func ToggleLikeSong(c *gin.Context, firestoreClient *firestore.Client) {
	uid, _ := c.Get("uid")

	var req struct {
		SongID string `json:"songId"`
		Action string `json:"action"` // "like" or "unlike"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload format"})
		return
	}

	ctx := context.Background()
	songRef := firestoreClient.Collection("songs").Doc(req.SongID)
	userLikeRef := firestoreClient.Collection("users").Doc(uid.(string)).Collection("liked_songs").Doc(req.SongID)

	var incrementValue int64 = 1
	if req.Action == "unlike" {
		incrementValue = -1
	}

	// Run an atomic transaction to ensure synchronization between user lists and global tallies
	err := firestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		var txErr error
		if req.Action == "like" {
			doc, err := tx.Get(songRef)
			if err != nil {
				return err
			}
			songData := doc.Data()
			songData["likedAt"] = time.Now() // Basis for library chronological listening sorting

			txErr = tx.Set(userLikeRef, songData)
			if txErr != nil {
				return txErr
			}
		} else {
			txErr = tx.Delete(userLikeRef)
			if txErr != nil {
				return txErr
			}
		}

		return tx.Update(songRef, []firestore.Update{
			{Path: "likes", Value: firestore.Increment(incrementValue)},
		})
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Transaction failed: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Like status successfully synchronized"})
}

// FollowArtist sets up clean, direct metrics mapping for specific singers
func FollowArtist(c *gin.Context, firestoreClient *firestore.Client) {
	uid, _ := c.Get("uid")
	var req struct {
		ArtistID string `json:"artistId"`
		Action   string `json:"action"` // "follow" or "unfollow"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload format"})
		return
	}

	ctx := context.Background()
	userFollowRef := firestoreClient.Collection("users").Doc(uid.(string)).Collection("following_artists").Doc(req.ArtistID)

	var err error
	if req.Action == "follow" {
		_, err = userFollowRef.Set(ctx, map[string]interface{}{
			"artistId":   req.ArtistID,
			"followedAt": time.Now(),
		})
	} else {
		_, err = userFollowRef.Delete(ctx)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update connection matrix"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Artist relationship table successfully mutated"})
}

// GetTopSongs builds high-performance ranking tables for the home page chart carousels
func GetTopSongs(c *gin.Context, firestoreClient *firestore.Client) {
	ctx := context.Background()

	// Strictly limiting retrieval size to optimize transmission and memory bounds
	docs, err := firestoreClient.Collection("songs").
		OrderBy("playCount", firestore.Desc).
		Limit(20).
		Documents(ctx).GetAll()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate algorithmic metrics"})
		return
	}

	songs := []map[string]interface{}{}
	for _, doc := range docs {
		data := doc.Data()
		if ts, ok := data["uploadedAt"].(time.Time); ok {
			data["uploadedAt"] = ts.Unix()
		}
		songs = append(songs, data)
	}
	c.JSON(http.StatusOK, gin.H{"results": songs})
}

// GetSongsByYearRange fetches timelines directly using numeric bounds filtering
func GetSongsByYearRange(c *gin.Context, firestoreClient *firestore.Client) {
	ctx := context.Background()
	startStr := c.Query("start")
	endStr := c.Query("end")

	if startStr == "" || endStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Start and end boundaries are required"})
		return
	}

	startYear, _ := strconv.Atoi(startStr)
	endYear, _ := strconv.Atoi(endStr)

	docs, err := firestoreClient.Collection("songs").
		Where("createdYear", ">=", startYear).
		Where("createdYear", "<=", endYear).
		OrderBy("createdYear", firestore.Desc).
		Documents(ctx).GetAll()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch historical range: %v", err)})
		return
	}

	songs := []map[string]interface{}{}
	for _, doc := range docs {
		data := doc.Data()
		if ts, ok := data["uploadedAt"].(time.Time); ok {
			data["uploadedAt"] = ts.Unix()
		}
		songs = append(songs, data)
	}
	c.JSON(http.StatusOK, gin.H{"songs": songs})
}

// DownloadSong increments the storage counter metrics and outputs explicit asset metadata mappings
func DownloadSong(c *gin.Context, firestoreClient *firestore.Client, storageService *services.StorageService) {
	songID := c.Query("songId")
	if songID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Target Song ID metric query param required"})
		return
	}

	ctx := context.Background()
	songRef := firestoreClient.Collection("songs").Doc(songID)

	// Fetch clean target details to verify item presence and grab storage layout keys
	doc, err := songRef.Get(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Requested track asset missing from records"})
		return
	}

	songData := doc.Data()
	fileName, ok := songData["fileName"].(string)
	if !ok || fileName == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "File system link damaged on document meta data profile"})
		return
	}

	// Dynamic asset authorization URL provisioning generation (e.g., valid for 1 hour download lifespan)
	downloadSignedURL, err := storageService.GenerateDownloadURL(fileName, 3600)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed storage provider pipeline handshake: %v", err)})
		return
	}

	// Atomic allocation execution to increment systemic analytical tallies safely
	_, _ = songRef.Update(ctx, []firestore.Update{
		{Path: "downloads", Value: firestore.Increment(1)},
	})

	// Clean output mapping profile containing all required download verification markers
	c.JSON(http.StatusOK, gin.H{
		"songId":      songID,
		"title":       songData["title"],
		"fileName":    fileName,
		"downloadUrl": downloadSignedURL, // The asset pipeline string your RN Client writes to disk
		"fileSize":    songData["fileSize"],
	})
}
