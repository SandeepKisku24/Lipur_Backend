package controllers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
	"google.golang.org/api/iterator"
)

// Helper function to safely convert []interface{} to []string
func interfaceArrayToStringArray(data interface{}) []string {
	if arr, ok := data.([]interface{}); ok {
		strArr := make([]string, len(arr))
		for i, v := range arr {
			if s, ok := v.(string); ok {
				strArr[i] = s
			} else {
				strArr[i] = fmt.Sprintf("%v", v)
			}
		}
		return strArr
	}
	return []string{}
}

// Helper function to safely extract integer fields (handling float64/int64)
func safeInt(data interface{}) int {
	if v, ok := data.(int64); ok {
		return int(v) // Handles int64 (default for whole numbers from Firestore)
	}
	if v, ok := data.(float64); ok {
		return int(v) // Handles float64
	}
	return 0
}

// SearchResult represents a unified structure for the frontend
type SearchResult struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	ArtistNames []string `json:"artistNames"`
	Artwork     string   `json:"artwork"`
	URL         string   `json:"url"`
	Duration    int      `json:"duration,omitempty"`
	Type        string   `json:"type"` // "song" or "artist"
}

func Search(c *gin.Context, firestoreClient *firestore.Client) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusOK, gin.H{"results": []SearchResult{}})
		return
	}

	ctx := context.Background()
	results := make([]SearchResult, 0)
	searchTermLower := strings.ToLower(query)
	endTerm := searchTermLower + "\uf8ff"

	// --- 1. Search Songs by Normalized Title Prefix (searchTitle) ---

	songQuery := firestoreClient.Collection("songs").
		OrderBy("searchTitle", firestore.Asc).
		StartAt(searchTermLower).
		EndAt(endTerm)

	iter := songQuery.Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("Error fetching songs: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query songs"})
			return
		}

		data := doc.Data()

		// Map song data to SearchResult structure
		results = append(results, SearchResult{
			ID:    data["id"].(string),
			Title: data["title"].(string),
			// FIX: Use safe helper for array conversion
			ArtistNames: interfaceArrayToStringArray(data["artistNames"]),

			// FIX: Use safe helper for numeric conversion
			URL:      data["fileUrl"].(string),
			Artwork:  data["coverUrl"].(string),
			Duration: safeInt(data["duration"]),
			Type:     "song",
		})
	}

	// --- 2. Search Artists by Normalized Name Prefix (searchName) ---

	artistQuery := firestoreClient.Collection("artists").
		// FIX: Query the normalized field 'searchName'
		OrderBy("searchName", firestore.Asc).
		StartAt(searchTermLower).
		EndAt(endTerm)

	artistIter := artistQuery.Documents(ctx)
	for {
		doc, err := artistIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("Error fetching artists: %v", err)
			break
		}

		data := doc.Data()

		// Use the original 'name' for display
		name, ok := data["name"].(string)
		if !ok {
			name = "Unknown Artist"
		}

		// Assume profileImageUrl might not exist, provide a fallback or check
		artwork := ""
		if img, ok := data["profileImageUrl"].(string); ok {
			artwork = img
		}

		results = append(results, SearchResult{
			ID:          data["id"].(string),
			Title:       name,
			ArtistNames: []string{name}, // Array containing just the artist's name
			URL:         "",             // Artists don't have a direct URL
			Artwork:     artwork,
			Duration:    0,
			Type:        "artist",
		})
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}

func GetSongsByArtist(c *gin.Context, firestoreClient *firestore.Client) {
	ctx := context.Background()
	artistID := c.Query("artistId") // Expecting /songs-by-artist?artistId=...

	if artistID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Artist ID is required."})
		return
	}

	// Query songs where the artistIds array contains the requested ID.
	// NOTE: This requires a composite index on (artistIds, uploadedAt).
	docs, err := firestoreClient.Collection("songs").
		Where("artistIds", "array-contains", artistID).
		OrderBy("uploadedAt", firestore.Desc).
		Documents(ctx).GetAll()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch artist's songs: %v", err)})
		return
	}

	songs := []map[string]interface{}{}
	for _, doc := range docs {
		data := doc.Data()
		// Convert timestamp (optional, but good for consistency)
		if ts, ok := data["uploadedAt"].(time.Time); ok {
			data["uploadedAt"] = ts.Unix()
		}
		songs = append(songs, data)
	}

	c.JSON(http.StatusOK, gin.H{"songs": songs})
}
