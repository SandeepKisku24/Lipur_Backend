package routes

import (
	"lipur_backend/controllers"
	"lipur_backend/services"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, storageService *services.StorageService) {
	r.POST("/upload", func(c *gin.Context) {
		controllers.UploadSong(c, storageService)
	})
}
