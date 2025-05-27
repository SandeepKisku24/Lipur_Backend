package main

import (
	"fmt"
	"lipur_backend/config"
	"lipur_backend/routes"
	"lipur_backend/services"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	config.LoadEnv()

	service := services.NewStorageService()
	if service.AccountID == "" || service.AppKey == "" || service.BucketName == "" {
		log.Fatal("Missing required env vars: B2_ACCOUNT_ID, B2_APPLICATION_KEY, or B2_BUCKET_NAME")
	}

	fmt.Println("AccountID inside service:", service.AccountID)

	r := gin.Default()
	routes.RegisterRoutes(r, service)

	port := config.GetEnv("PORT")
	log.Println("Server running on port:", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
