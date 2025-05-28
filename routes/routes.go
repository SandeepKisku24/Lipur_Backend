package routes

import (
	"lipur_backend/controllers"
	"lipur_backend/services"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, storageService *services.StorageService, s3Client *services.S3Client, firestoreClient *firestore.Client) {
	r.POST("/upload", func(c *gin.Context) {
		controllers.UploadSong(c, storageService, firestoreClient)
	})
	r.GET("/stream-url", func(c *gin.Context) {
		controllers.GetSignedMusicURL(c, storageService)
	})
	r.GET("/songs", func(c *gin.Context) {
		controllers.GetSongs(c, firestoreClient)
	})
}
