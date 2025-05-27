package routes

import (
	"lipur_backend/controllers"
	"lipur_backend/services"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, storageService *services.StorageService, s3Client *services.S3Client) {
	r.POST("/upload", func(c *gin.Context) {
		controllers.UploadSong(c, storageService)
	})
	r.GET("/stream-url", func(c *gin.Context) {
		controllers.GetSignedMusicURL(c, storageService)
	})
}
