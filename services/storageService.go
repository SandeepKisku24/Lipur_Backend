package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

type StorageService struct {
	AccountID      string // FULL ID (from ENV)
	ShortAccountID string // SHORT ID (from auth response)
	AppKey         string
	BucketName     string

	AuthToken       string
	APIUrl          string
	DownloadUrl     string
	UploadUrl       string
	UploadAuthToken string
}

func NewStorageService() *StorageService {
	accountID := os.Getenv("B2_ACCOUNT_ID") // FULL ID, e.g. 005448b895ade3d0000000004
	appKey := os.Getenv("B2_APPLICATION_KEY")
	bucketName := os.Getenv("B2_BUCKET_NAME")

	fmt.Println("Loaded ENV values:")
	fmt.Println("AccountID (from ENV):", accountID)
	fmt.Println("AppKey (partial):", appKey[:5]+"...")
	fmt.Println("BucketName:", bucketName)

	return &StorageService{
		AccountID:  accountID,
		AppKey:     appKey,
		BucketName: bucketName,
	}
}

type authResponse struct {
	AuthorizationToken string `json:"authorizationToken"`
	APIUrl             string `json:"apiUrl"`
	DownloadUrl        string `json:"downloadUrl"`
	AccountId          string `json:"accountId"` // SHORT ID from response
}

func (s *StorageService) Authenticate() error {
	req, err := http.NewRequest("GET", "https://api.backblazeb2.com/b2api/v2/b2_authorize_account", nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(s.AccountID, s.AppKey)

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		bodyBytes, _ := ioutil.ReadAll(res.Body)
		return fmt.Errorf("failed to authorize account: %s", string(bodyBytes))
	}

	var authRes authResponse
	if err := json.NewDecoder(res.Body).Decode(&authRes); err != nil {
		return err
	}

	s.AuthToken = authRes.AuthorizationToken
	s.APIUrl = authRes.APIUrl
	s.DownloadUrl = authRes.DownloadUrl
	s.ShortAccountID = authRes.AccountId // This is the short accountId
	fmt.Println("AuthToken being used:", s.AuthToken)

	fmt.Println("Authenticated! API URL:", s.APIUrl)
	fmt.Println("Short Account ID:", s.ShortAccountID)
	return nil
}

type bucketListResponse struct {
	Buckets []struct {
		BucketID   string `json:"bucketId"`
		BucketName string `json:"bucketName"`
	} `json:"buckets"`
}

func (s *StorageService) getBucketID() (string, error) {
	bodyMap := map[string]string{
		"accountId": s.ShortAccountID, // <-- Use SHORT account ID here!
	}
	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", s.APIUrl+"/b2api/v2/b2_list_buckets", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", s.AuthToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		bodyBytes, _ := ioutil.ReadAll(res.Body)
		return "", fmt.Errorf("list buckets failed: %s", string(bodyBytes))
	}

	var bucketRes bucketListResponse
	if err := json.NewDecoder(res.Body).Decode(&bucketRes); err != nil {
		return "", err
	}

	for _, b := range bucketRes.Buckets {
		if b.BucketName == s.BucketName {
			log.Println("Found bucket:", b.BucketName, "with ID:", b.BucketID)
			return b.BucketID, nil
		}
	}

	return "", errors.New("bucket not found")
}

type uploadUrlResponse struct {
	UploadUrl          string `json:"uploadUrl"`
	AuthorizationToken string `json:"authorizationToken"`
}

func (s *StorageService) getUploadURL() error {
	bucketID, err := s.getBucketID()
	if err != nil {
		return err
	}

	reqBody := []byte(fmt.Sprintf(`{"bucketId":"%s"}`, bucketID))
	req, err := http.NewRequest("POST", s.APIUrl+"/b2api/v2/b2_get_upload_url", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", s.AuthToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		bodyBytes, _ := ioutil.ReadAll(res.Body)
		return fmt.Errorf("get upload url failed: %s", string(bodyBytes))
	}

	var uploadRes uploadUrlResponse
	if err := json.NewDecoder(res.Body).Decode(&uploadRes); err != nil {
		return err
	}

	s.UploadUrl = uploadRes.UploadUrl
	s.UploadAuthToken = uploadRes.AuthorizationToken

	log.Println("Got upload URL:", s.UploadUrl)
	return nil
}

func (s *StorageService) UploadFile(ctx context.Context, filename string, data []byte) (string, error) {
	// Authenticate if needed
	if s.AuthToken == "" || s.APIUrl == "" || s.ShortAccountID == "" {
		if err := s.Authenticate(); err != nil {
			return "", err
		}
	}

	// Get upload URL if needed
	if s.UploadUrl == "" || s.UploadAuthToken == "" {
		if err := s.getUploadURL(); err != nil {
			return "", err
		}
	}
	safeFileName := url.PathEscape(filename)

	req, err := http.NewRequest("POST", s.UploadUrl, bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", s.UploadAuthToken)
	req.Header.Set("X-Bz-File-Name", safeFileName) // <-- fixed here
	req.Header.Set("Content-Type", "b2/x-auto")
	req.Header.Set("X-Bz-Content-Sha1", "do_not_verify")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(data)))

	client := &http.Client{Timeout: 60 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		bodyBytes, _ := ioutil.ReadAll(res.Body)
		return "", fmt.Errorf("upload failed: %s", string(bodyBytes))
	}

	publicUrl := fmt.Sprintf("%s/file/%s/%s", s.DownloadUrl, s.BucketName, filename)
	return publicUrl, nil
}

func (s *StorageService) GenerateDownloadURL(fileName string, validDurationSeconds int) (string, error) {
	if s.AuthToken == "" || s.APIUrl == "" || s.ShortAccountID == "" {
		if err := s.Authenticate(); err != nil {
			return "", err
		}
	}

	bucketID, err := s.getBucketID()
	if err != nil {
		return "", err
	}

	requestBody := map[string]interface{}{
		"bucketId":               bucketID,
		"fileNamePrefix":         fileName,
		"validDurationInSeconds": validDurationSeconds,
	}

	body, _ := json.Marshal(requestBody)

	req, err := http.NewRequest("POST", s.APIUrl+"/b2api/v2/b2_get_download_authorization", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", s.AuthToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
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
		return "", err
	}

	// Construct pre-signed URL
	url := fmt.Sprintf(
		"%s/file/%s/%s?Authorization=%s",
		s.DownloadUrl,
		s.BucketName,
		fileName,
		authResp.AuthorizationToken,
	)

	return url, nil
}
