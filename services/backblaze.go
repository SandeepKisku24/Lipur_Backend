package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Client struct {
	s3Client      *s3.Client
	presignClient *s3.PresignClient
}

// NewS3Client initializes the S3 client for Backblaze B2
func NewS3Client() (*S3Client, error) {
	// Validate environment variables
	accountID := os.Getenv("B2_ACCOUNT_ID")
	appKey := os.Getenv("B2_APPLICATION_KEY")
	region := os.Getenv("B2_REGION")
	endpoint := os.Getenv("B2_ENDPOINT")
	if accountID == "" || appKey == "" || region == "" || endpoint == "" {
		return nil, fmt.Errorf("missing required env vars: B2_ACCOUNT_ID, B2_APPLICATION_KEY, B2_REGION, or B2_ENDPOINT")
	}

	// Load AWS SDK configuration for Backblaze B2
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accountID,
			appKey,
			"",
		)),
		config.WithRegion(region),
		config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               "https://" + endpoint,
					SigningRegion:     region,
					HostnameImmutable: true,
				}, nil
			}),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg)
	return &S3Client{
		s3Client:      client,
		presignClient: s3.NewPresignClient(client),
	}, nil
}

// GenerateSignedURL creates a pre-signed URL using B2 Native API
func (s *StorageService) GenerateSignedURL(fileName string) (string, error) {
	if s.AuthToken == "" || s.APIUrl == "" || s.ShortAccountID == "" {
		if err := s.Authenticate(); err != nil {
			return "", fmt.Errorf("failed to authenticate: %w", err)
		}
	}

	bucketID, err := s.getBucketID()
	if err != nil {
		return "", fmt.Errorf("failed to get bucket ID: %w", err)
	}

	requestBody := map[string]interface{}{
		"bucketId":               bucketID,
		"fileNamePrefix":         fileName,
		"validDurationInSeconds": 3600, // 1 hour
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest("POST", s.APIUrl+"/b2api/v2/b2_get_download_authorization", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", s.AuthToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get download authorization: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("download auth failed: %s", string(bodyBytes))
	}

	var authResp struct {
		AuthorizationToken string `json:"authorizationToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Construct B2 Native API signed URL
	signedURL := fmt.Sprintf(
		"%s/file/%s/%s?Authorization=%s",
		s.DownloadUrl,
		s.BucketName,
		url.PathEscape(fileName),
		url.QueryEscape(authResp.AuthorizationToken),
	)

	log.Printf("Generated B2 Native signed URL for %s: %s", fileName, signedURL)
	return signedURL, nil
}

// package services

// import (
// 	"context"
// 	"fmt"
// 	"log"
// 	"os"
// 	"time"

// 	"github.com/aws/aws-sdk-go-v2/aws"
// 	"github.com/aws/aws-sdk-go-v2/config"
// 	"github.com/aws/aws-sdk-go-v2/credentials"
// 	"github.com/aws/aws-sdk-go-v2/service/s3"
// )

// type S3Client struct {
// 	s3Client      *s3.Client
// 	presignClient *s3.PresignClient
// }

// // NewS3Client initializes the S3 client for Backblaze B2
// func NewS3Client() (*S3Client, error) {
// 	// Validate environment variables
// 	accountID := os.Getenv("B2_ACCOUNT_ID")
// 	appKey := os.Getenv("B2_APPLICATION_KEY")
// 	region := os.Getenv("B2_REGION")
// 	endpoint := os.Getenv("B2_ENDPOINT")
// 	if accountID == "" || appKey == "" || region == "" || endpoint == "" {
// 		return nil, fmt.Errorf("missing required env vars: B2_ACCOUNT_ID, B2_APPLICATION_KEY, B2_REGION, or B2_ENDPOINT")
// 	}

// 	// Load AWS SDK configuration for Backblaze B2
// 	cfg, err := config.LoadDefaultConfig(context.TODO(),
// 		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
// 			accountID,
// 			appKey,
// 			"",
// 		)),
// 		config.WithRegion(region),
// 		config.WithEndpointResolverWithOptions(
// 			aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
// 				return aws.Endpoint{
// 					URL:               "https://" + endpoint,
// 					SigningRegion:     region,
// 					HostnameImmutable: true,
// 				}, nil
// 			}),
// 		),
// 	)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to load AWS config: %w", err)
// 	}

// 	client := s3.NewFromConfig(cfg)
// 	return &S3Client{
// 		s3Client:      client,
// 		presignClient: s3.NewPresignClient(client),
// 	}, nil
// }

// // GenerateSignedURL creates a pre-signed URL for streaming a file from Backblaze B2
// func (c *S3Client) GenerateSignedURL(fileName string) (string, error) {
// 	bucket := os.Getenv("B2_BUCKET_NAME")
// 	if bucket == "" || fileName == "" {
// 		return "", fmt.Errorf("bucket name or file name is empty")
// 	}

// 	// Generate pre-signed URL with streaming-friendly headers
// 	req, err := c.presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
// 		Bucket:              aws.String(bucket),
// 		Key:                 aws.String(fileName),
// 		ResponseContentType: aws.String("audio/mpeg"),
// 	}, s3.WithPresignExpires(1*time.Hour)) // 1-hour expiration for streaming
// 	if err != nil {
// 		return "", fmt.Errorf("failed to generate signed URL: %w", err)
// 	}

// 	log.Printf("Generated signed URL for %s: %s", fileName, req.URL)
// 	return req.URL, nil
// }
