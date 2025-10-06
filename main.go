package main

import (
	"context"
	"fmt"
	"lipur_backend/config"
	"lipur_backend/routes"
	"lipur_backend/services"
	"log"

	firebase "firebase.google.com/go"
	"github.com/gin-gonic/gin"
	"google.golang.org/api/option"
)

func main() {
	config.LoadEnv()

	ctx := context.Background()
	opt := option.WithCredentialsFile("serviceAccountKey.json")
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		log.Fatalf("Failed to initialize Firebase: %v", err)
	}
	firestoreClient, err := app.Firestore(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize Firestore: %v", err)
	}
	defer firestoreClient.Close()

	authClient, err := app.Auth(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize Firebase Auth: %v", err)
	}

	// Initialize S3 client for Backblaze B2
	s3Client, err := services.NewS3Client()
	if err != nil {
		log.Fatal("Failed to initialize S3 client:", err)
	}

	// Initialize storage service
	service := services.NewStorageService()
	if service.AccountID == "" || service.AppKey == "" || service.BucketName == "" {
		log.Fatal("Missing required env vars: B2_ACCOUNT_ID, B2_APPLICATION_KEY, or B2_BUCKET_NAME")
	}

	fmt.Println("AccountID inside service:", service.AccountID)

	r := gin.Default()
	routes.RegisterRoutes(r, service, s3Client, firestoreClient, authClient)

	port := config.GetEnv("PORT")
	log.Println("Server running on port:", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
