package controllers

import (
	"context"
	"fmt"
	"io"
	"lipur_backend/services"
	"log"
	"net/http"
	"net/url"
	"path"
	"time"

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
	createdYear := c.PostForm("createdYear")
	if createdYear == "" {
		createdYear = "time.Now().Format(\"2006\")"
	}
	upload_user := c.PostForm("upload_user")
	if upload_user == "" {
		upload_user = "admin"
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
		"id":          songId,
		"title":       title,
		"artistName":  artistName,
		"artistId":    artistId,
		"fileName":    filename,
		"fileUrl":     publicURL,
		"duration":    0,
		"genre":       genre,
		"uploadedAt":  time.Now(),
		"coverUrl":    coverUrl,
		"likes":       0,
		"downloads":   0,
		"playCount":   0,
		"createdYear": createdYear,
		"upload_user": upload_user,
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

	fullFileUrl := c.Query("file")
	if fullFileUrl == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File URL is required."})
		return
	}

	// 2. Parse the full URL
	parsedUrl, err := url.Parse(fullFileUrl)
	if err != nil {
		log.Printf("Error parsing file URL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid file URL format."})
		return
	}

	// 3. Extract the Object Key (Filename)
	// parsedUrl.Path gives us the URL path string (e.g., "/file/LipurMusic/Happier.mp3")
	// path.Base() returns the last element of the path (e.g., "Happier.mp3")
	// This is the robust way to extract the key regardless of the B2 structure.
	objectKey := path.Base(parsedUrl.Path) // <--- FINAL FIX: Use path.Base()

	if objectKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Could not determine file key from URL path."})
		return
	}

	// 4. Call the service with ONLY the object key (e.g., "Happier.mp3")
	url, err := storageService.GenerateSignedURL(objectKey)
	if err != nil {
		log.Printf("Failed to generate signed URL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate signed URL: %v", err)})
		return
	}

	// 5. Response structure
	c.JSON(http.StatusOK, gin.H{"url": url})
	// fileName := c.Query("file")
	// if fileName == "" {
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": "File name is required to uppload "})
	// 	return
	// }

	// url, err := storageService.GenerateSignedURL(fileName)
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to generate signed URL: %v", err)})
	// 	return
	// }

	// c.JSON(http.StatusOK, gin.H{"url": url})
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

// for playlist and user
func CreatePlaylist(c *gin.Context, firestoreClient *firestore.Client) {
	userId, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err)})
		return
	}

	if request.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Playlist name is required"})
		return
	}

	playlistId := uuid.New().String()
	playlist := map[string]interface{}{
		"id":          playlistId,
		"name":        request.Name,
		"description": request.Description,
		"songs":       []map[string]interface{}{},
		"createdAt":   time.Now(),
	}

	ctx := context.Background()
	_, err := firestoreClient.Collection("users").Doc(userId.(string)).Collection("playlists").Doc(playlistId).Set(ctx, playlist)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create playlist: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Playlist created",
		"playlistId": playlistId,
	})
}

func GetPlaylists(c *gin.Context, firestoreClient *firestore.Client) {
	userId, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	ctx := context.Background()
	docs, err := firestoreClient.Collection("users").Doc(userId.(string)).Collection("playlists").OrderBy("createdAt", firestore.Desc).Documents(ctx).GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch playlists: %v", err)})
		return
	}

	playlists := []map[string]interface{}{}
	for _, doc := range docs {
		data := doc.Data()
		if ts, ok := data["createdAt"].(time.Time); ok {
			data["createdAt"] = ts.Unix()
		}
		playlists = append(playlists, data)
	}

	c.JSON(http.StatusOK, gin.H{"playlists": playlists})
}

func AddSongToPlaylist(c *gin.Context, firestoreClient *firestore.Client) {
	userId, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	playlistId := c.Param("id")
	if playlistId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Playlist ID is required"})
		return
	}

	var request struct {
		SongId string `json:"songId"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err)})
		return
	}

	// Fetch song details
	ctx := context.Background()
	songDoc, err := firestoreClient.Collection("songs").Doc(request.SongId).Get(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Song not found: %v", err)})
		return
	}
	songData := songDoc.Data()

	// Update playlist
	playlistRef := firestoreClient.Collection("users").Doc(userId.(string)).Collection("playlists").Doc(playlistId)
	_, err = playlistRef.Update(ctx, []firestore.Update{
		{Path: "songs", Value: firestore.ArrayUnion(songData)},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to add song to playlist: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Song added to playlist"})
}
