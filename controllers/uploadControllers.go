package controllers

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"lipur_backend/services"

	"github.com/gin-gonic/gin"
)

func UploadSong(c *gin.Context, storageService *services.StorageService) {
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
	data, err := ioutil.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read file: %v", err)})
		return
	}

	// Log file details
	fmt.Printf("Uploading file: %s, size: %d bytes\n", file.Filename, len(data))

	// Upload to Backblaze B2
	ctx := context.Background()
	publicURL, err := storageService.UploadFile(ctx, file.Filename, data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to upload file: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "File uploaded successfully", "url": publicURL})
}

func GetSignedMusicURL(c *gin.Context, storageService *services.StorageService) {
	fileName := c.Query("file")
	if fileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File name is required"})
		return
	}

	url, err := storageService.GenerateSignedURL(fileName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to generate signed URL: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": url})
}
