package routes

import (
	"lipur_backend/controllers"
	"lipur_backend/middleware"
	"lipur_backend/services"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/auth"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, storageService *services.StorageService, s3Client *services.S3Client, firestoreClient *firestore.Client, authClient *auth.Client) {
	r.POST("/upload", func(c *gin.Context) {
		controllers.UploadSong(c, storageService, firestoreClient)
	})
	r.GET("/stream-url", func(c *gin.Context) {
		controllers.GetSignedMusicURL(c, storageService)
	})
	r.GET("/songs", func(c *gin.Context) {
		controllers.GetSongs(c, firestoreClient)
	})

	// for users
	// User routes (public for registration and login)
	r.POST("/register", func(c *gin.Context) {
		controllers.RegisterUser(c, firestoreClient, authClient)
	})
	r.POST("/login", func(c *gin.Context) {
		controllers.LoginUser(c, authClient)
	})

	// Protected routes
	protected := r.Group("/").Use(middleware.AuthMiddleware(authClient))
	{
		protected.GET("/users", func(c *gin.Context) {
			controllers.ListUsers(c, firestoreClient)
		})
		protected.POST("/playlists", func(c *gin.Context) {
			controllers.CreatePlaylist(c, firestoreClient)
		})
		protected.GET("/playlists", func(c *gin.Context) {
			controllers.GetPlaylists(c, firestoreClient)
		})
		protected.POST("/playlists/:id/songs", func(c *gin.Context) {
			controllers.AddSongToPlaylist(c, firestoreClient)
		})
		r.POST("/admin/migrate-artists", func(c *gin.Context) {
			controllers.MigrateArtistIDs(c, firestoreClient)
		})
	}

}
