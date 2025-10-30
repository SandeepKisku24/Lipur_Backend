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
	"strings"
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

	searchTitle := strings.ToLower(strings.TrimSpace(title))
	artistNames := c.PostFormArray("artists") // Array of artist names
	log.Printf("Received artist names: %v", artistNames)
	if len(artistNames) == 0 {
		artistNames = []string{"Unknown Artist"}
	}
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
	finalArtistNames := []string{}
	finalArtistIds := []string{}
	log.Println("Starting artist processing...", len(artistNames))
	// 1. Iterate through all submitted artist names
	artistRef := firestoreClient.Collection("artists")
	for _, artistName := range artistNames {
		artistName = strings.TrimSpace(artistName)
		if artistName == "" {
			continue
		}

		// --- Normalization (The key for preventing duplication) ---
		normalizedArtistName := strings.ToLower(artistName)
		// -----------------------------------------------------------

		// 1. Check if artist already exists using the **NORMALIZED** name/field.
		// This prevents duplication by matching 'stephan tudu' == 'Stephan Tudu'.
		query := artistRef.Where("searchName", "==", normalizedArtistName).Limit(1)
		docs, err := query.Documents(ctx).GetAll()

		if err != nil {
			log.Printf("Error querying artist %s: %v", artistName, err)
			continue
		}

		var artistDocId string

		if len(docs) > 0 {
			// Case A: Artist Found - Reuse the stable Firestore Document ID
			artistDocId = docs[0].Ref.ID
			log.Printf("Artist found via normalized name: %s, ID: %s", artistName, artistDocId)

		} else {
			// Case B: Artist Not Found - Create new record
			artistDocId = uuid.New().String()

			_, setErr := artistRef.Doc(artistDocId).Set(ctx, map[string]interface{}{
				"name":            artistName,           // Retain original case for display
				"searchName":      normalizedArtistName, // <--- NEW: Normalized field for future lookups
				"bio":             "",
				"profileImageUrl": "",
				"createdAt":       time.Now(),
				"id":              artistDocId,
			})
			if setErr != nil {
				log.Printf("Warning: Failed to save NEW artist %s: %v", artistName, setErr)
			}
			log.Printf("New artist created: %s, ID: %s", artistName, artistDocId)
		}

		// 3. Append the canonical Firestore Document ID and Name for song metadata
		finalArtistNames = append(finalArtistNames, artistName)
		finalArtistIds = append(finalArtistIds, artistDocId)
	}

	// Save song metadata to Firestore
	songId := uuid.New().String()
	metadata := map[string]interface{}{
		"id":          songId,
		"title":       title,
		"artistNames": finalArtistNames,
		"artistIds":   finalArtistIds,
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
		"searchTitle": searchTitle,
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

type SongDataStructure struct {
	ID          string   `firestore:"id"`
	ArtistNames []string `firestore:"artistNames"` // Array of names
	ArtistIds   []string `firestore:"artistIds"`   // Array of IDs (potentially bad)
}

type ArtistDataStructure struct {
	ID   string `firestore:"id"`
	Name string `firestore:"name"`
}

// MigrateArtistIDs performs a full cleanup and normalization of artist IDs.
// This should ONLY be run by an administrator.
func MigrateArtistIDs(c *gin.Context, firestoreClient *firestore.Client) {
	ctx := context.Background()
	log.Println("Starting full Artist ID Normalization and Migration...")

	// Key: Clean Lowercase Artist Name (string), Value: Canonical Stable Artist ID (string)
	canonicalIDMap := make(map[string]string)

	// Total updates tracker
	var songsUpdated int

	// --- 1. STAGE 1: Normalize Artists Collection (Establish Canonical IDs) ---

	artistDocs, err := firestoreClient.Collection("artists").Documents(ctx).GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch artists: %v", err)})
		return
	}

	artistBatch := firestoreClient.Batch()

	// Separate batch to delete duplicates in a separate transaction if needed, but we merge into one for efficiency
	var deletedArtistCount int

	for _, doc := range artistDocs {
		name, ok := doc.Data()["name"].(string)
		if !ok || name == "" {
			log.Printf("Warning: Artist document %s has invalid or missing name. Deleting.", doc.Ref.ID)
			artistBatch.Delete(doc.Ref)
			continue
		}

		// FIX 1: Normalize the name: lowercase and trim whitespace for the map key
		cleanName := strings.ToLower(strings.TrimSpace(name))

		if stableID, exists := canonicalIDMap[cleanName]; exists {
			// Case A: Duplicate found. Mark the current document for deletion.
			log.Printf("Duplicate artist found: %s. Doc %s marked for deletion, using canonical ID %s.", cleanName, doc.Ref.ID, stableID)
			artistBatch.Delete(doc.Ref)
			deletedArtistCount++
			continue
		}

		// Case B: First time encountering this artist. Establish this ID as canonical.
		var canonicalID string
		// Use document ID as canonical ID, unless it's obviously bad (e.g., auto-generated short ID)
		if len(doc.Ref.ID) < 10 {
			canonicalID = uuid.New().String()
		} else {
			canonicalID = doc.Ref.ID
		}

		// Add to map using the CLEANED name
		canonicalIDMap[cleanName] = canonicalID

		// Update the document to ensure 'id' field is present and consistent with Doc ID
		artistBatch.Set(doc.Ref, map[string]interface{}{
			"id":        canonicalID,
			"name":      name,
			"createdAt": time.Now(),
		}, firestore.MergeAll)
	}

	// Commit Artist Normalization Batch
	_, err = artistBatch.Commit(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Artist normalization failed: %v", err)})
		return
	}

	log.Printf("Artist normalization successful. %d artists stabilized. %d duplicates deleted. Starting song migration...", len(canonicalIDMap), deletedArtistCount)

	// --- 2. STAGE 2: Update Song Documents ---

	songDocs, err := firestoreClient.Collection("songs").Documents(ctx).GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch songs for update: %v", err)})
		return
	}

	songBatch := firestoreClient.Batch()

	for _, doc := range songDocs {
		var song SongDataStructure
		doc.DataTo(&song)

		var newArtistIds []string

		// Process only if ArtistNames exist
		if len(song.ArtistNames) == 0 {
			song.ArtistNames = []string{"Unknown Artist"}
		}

		// Build the new array of IDs based on the stable name map
		for _, name := range song.ArtistNames {
			// FIX 2: Normalize the name before lookup
			cleanName := strings.ToLower(strings.TrimSpace(name))

			if stableID, ok := canonicalIDMap[cleanName]; ok {
				newArtistIds = append(newArtistIds, stableID) // Use the stable ID
			} else {
				// Fallback: If the artist name exists in the song but wasn't in the collection
				// (e.g., if the artist document was deleted), create a new unique ID for consistency.
				newArtistIds = append(newArtistIds, uuid.New().String())
			}
		}

		// Only update if the content has changed
		// Note: Comparing arrays by converting to string is simple but relies on element order
		// We only compare length for simplicity here.
		if len(newArtistIds) > 0 && len(newArtistIds) != len(song.ArtistIds) {
			songsUpdated++
		}

		// Always perform the update to ensure the structure is clean and IDs are consistent
		songBatch.Update(doc.Ref, []firestore.Update{
			{Path: "artistIds", Value: newArtistIds},
			{Path: "artistNames", Value: song.ArtistNames}, // Re-save name array for consistency
		})
	}

	// Commit Song Update Batch
	_, err = songBatch.Commit(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Song batch migration failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":             "Artist IDs successfully normalized and songs updated.",
		"artists_stabilized":  len(canonicalIDMap),
		"duplicates_deleted":  deletedArtistCount,
		"songs_updated_count": songsUpdated,
	})
}

func NormalizeSearchFields(c *gin.Context, firestoreClient *firestore.Client) {
	ctx := context.Background()
	log.Println("Starting Search Field Normalization...")

	var songsUpdated, artistsUpdated int

	// --- 1. Normalize Songs Collection ---
	songDocs, err := firestoreClient.Collection("songs").Documents(ctx).GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch songs: %v", err)})
		return
	}

	songBatch := firestoreClient.Batch()

	for _, doc := range songDocs {
		data := doc.Data()

		if title, ok := data["title"].(string); ok {
			searchTitle := strings.ToLower(strings.TrimSpace(title))

			songBatch.Update(doc.Ref, []firestore.Update{
				{Path: "searchTitle", Value: searchTitle}, // Add lowercase title
			})
			songsUpdated++
		}
	}

	_, err = songBatch.Commit(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Song title normalization failed: %v", err)})
		return
	}

	// --- 2. Normalize Artists Collection ---

	artistDocs, err := firestoreClient.Collection("artists").Documents(ctx).GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch artists: %v", err)})
		return
	}

	artistBatch := firestoreClient.Batch()

	for _, doc := range artistDocs {
		data := doc.Data()

		if name, ok := data["name"].(string); ok {
			searchName := strings.ToLower(strings.TrimSpace(name))

			// Use Batch Update to add the new field
			artistBatch.Update(doc.Ref, []firestore.Update{
				{Path: "searchName", Value: searchName}, // Add lowercase name
			})
			artistsUpdated++
		}
	}

	_, err = artistBatch.Commit(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Artist name normalization failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Search fields successfully normalized.",
		"songs_updated":   songsUpdated,
		"artists_updated": artistsUpdated,
	})
}
