package controllers

import (
	"fmt"
	"net/http"

	"lipur_backend/services"

	"github.com/gin-gonic/gin"
)

func UploadSong(c *gin.Context, storageService *services.StorageService) {
	if storageService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Storage service not initialized"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is required"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer f.Close()

	fileBytes := make([]byte, file.Size)
	_, err = f.Read(fileBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}

	url, err := storageService.UploadFile(c.Request.Context(), file.Filename, fileBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Upload failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": url})
}
