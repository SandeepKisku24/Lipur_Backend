package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func LoadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}
	log.Println("B2_ACCOUNT_ID:", os.Getenv("B2_ACCOUNT_ID"))
	log.Println("B2_APPLICATION_KEY:", os.Getenv("B2_APPLICATION_KEY")[:5]+"...")
	log.Println("B2_BUCKET_NAME:", os.Getenv("B2_BUCKET_NAME"))
	log.Println("B2_REGION:", os.Getenv("B2_REGION"))
	log.Println("B2_ENDPOINT:", os.Getenv("B2_ENDPOINT"))
	log.Println("PORT:", os.Getenv("PORT"))
}

func GetEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("Environment variable %s not set", key)
	}
	return val
}
