package controllers

import (
	"fmt"
	"net/http"

	"lipur_backend/services"

	"github.com/gin-gonic/gin"
)

func UploadSong(c *gin.Context, storageService *services.StorageService) {
	// ... (unchanged)
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
