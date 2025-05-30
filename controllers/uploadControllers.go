package controllers

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"lipur_backend/services"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func UploadSong(c *gin.Context, storageService *services.StorageService, firestoreClient *firestore.Client) {
	// Get file from form-data
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to get file: %v", err)})
		return
	}

	// Open file
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to open file: %v", err)})
		return
	}
	defer f.Close()

	// Read file data
	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read file: %v", err)})
		return
	}

	// Log file details
	log.Printf("Uploading file: %s, size: %d bytes", file.Filename, len(data))

	// Get metadata from form-data
	filename := file.Filename
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Filename is required"})
		return
	}
	title := c.PostForm("title")
	if title == "" {
		title = filename // Fallback to filename if title is empty
	}
	artistName := c.PostForm("artist")
	if artistName == "" {
		artistName = "Unknown Artist"
	}
	artistId := c.PostForm("artistId")
	genre := c.PostForm("genre")
	if genre == "" {
		genre = "Unknown"
	}
	coverUrl := c.PostForm("coverUrl")

	// Upload to Backblaze B2
	ctx := context.Background()
	publicURL, err := storageService.UploadFile(ctx, filename, data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to upload file: %v", err)})
		return
	}

	// Generate signed URL
	signedUrl, err := storageService.GenerateSignedURL(filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate signed URL: %v", err)})
		return
	}

	// Generate artistId if not provided
	if artistId == "" {
		artistId = uuid.New().String()
		_, err = firestoreClient.Collection("artists").Doc(artistId).Set(ctx, map[string]interface{}{
			"name":            artistName,
			"bio":             "",
			"profileImageUrl": "",
			"createdAt":       time.Now(),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save artist: %v", err)})
			return
		}
	}

	// Save song metadata to Firestore
	songId := uuid.New().String()
	metadata := map[string]interface{}{
		"id":         songId,
		"title":      title,
		"artistName": artistName,
		"artistId":   artistId,
		"fileName":   filename,
		"fileUrl":    publicURL,
		"duration":   0,
		"genre":      genre,
		"uploadedAt": time.Now(),
		"coverUrl":   coverUrl,
		"likes":      0,
		"downloads":  0,
		"playCount":  0,
	}

	_, err = firestoreClient.Collection("songs").Doc(songId).Set(ctx, metadata)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save metadata: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "File uploaded successfully",
		"songId":    songId,
		"publicUrl": publicURL,
		"signedUrl": signedUrl,
		"filename":  filename,
	})
}

func GetSignedMusicURL(c *gin.Context, storageService *services.StorageService) {
	fileName := c.Query("file")
	if fileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File name is required to uppload "})
		return
	}

	url, err := storageService.GenerateSignedURL(fileName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to generate signed URL: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": url})
}

func GetSongs(c *gin.Context, firestoreClient *firestore.Client) {
	ctx := context.Background()
	docs, err := firestoreClient.Collection("songs").OrderBy("uploadedAt", firestore.Desc).Documents(ctx).GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch songs: %v", err)})
		return
	}

	log.Printf("Fetched %d songs from Firestore", len(docs))

	songs := []map[string]interface{}{}
	for _, doc := range docs {
		data := doc.Data()
		// Convert Firestore timestamp (time.Time) to Unix seconds
		if ts, ok := data["uploadedAt"].(time.Time); ok {
			data["uploadedAt"] = ts.Unix()
		}
		songs = append(songs, data)
	}

	c.JSON(http.StatusOK, gin.H{"songs": songs})
}
