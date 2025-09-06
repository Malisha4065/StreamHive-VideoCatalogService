package services

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"
	"strconv"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/sony/gobreaker"
)

// Helper function to read secret from file or fallback to environment variable
func getSecret(filePath, envVar string) string {
	if data, err := ioutil.ReadFile(filePath); err == nil {
		return strings.TrimSpace(string(data))
	}
	return os.Getenv(envVar)
}

// AzureClientAdapter wraps Azure storage client for video deletion
type AzureClientAdapter struct {
	service   *azblob.Client
	container string
	breaker   *gobreaker.CircuitBreaker
}

// NewAzureClientAdapterFromEnv creates an Azure client from environment variables
func NewAzureClientAdapterFromEnv() (*AzureClientAdapter, error) {
	container := getSecret("/mnt/secrets-store/azure-storage-raw-container", "AZURE_BLOB_CONTAINER")
	if container == "" {
		container = "uploadservicecontainer"
	}

	acct := getSecret("/mnt/secrets-store/azure-storage-account", "AZURE_STORAGE_ACCOUNT")
	connStr := getSecret("/mnt/secrets-store/azure-storage-connection-string", "AZURE_STORAGE_CONNECTION_STRING")
	key := getSecret("/mnt/secrets-store/azure-storage-key", "AZURE_STORAGE_KEY")

	var svc *azblob.Client
	var err error

	// Try connection string first
	if connStr != "" {
		svc, err = azblob.NewClientFromConnectionString(connStr, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create client from connection string: %w", err)
		}
	} else if acct != "" && key != "" {
		// Use account + key
		cred, err := azblob.NewSharedKeyCredential(acct, key)
		if err != nil {
			return nil, fmt.Errorf("failed to create credentials: %w", err)
		}
		serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", acct)
		svc, err = azblob.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create client: %w", err)
		}
	} else {
		return nil, fmt.Errorf("missing Azure storage credentials - need either AZURE_STORAGE_CONNECTION_STRING or AZURE_STORAGE_ACCOUNT+AZURE_STORAGE_KEY")
	}

	// Circuit breaker settings from env (optional)
	cbTimeout := 10 * time.Second
	if v := os.Getenv("CATALOG_CB_RESET_MS"); v != "" {
		if d, err := time.ParseDuration(v + "ms"); err == nil { cbTimeout = d }
	}
	cbFailures := uint32(5)
	if v := os.Getenv("CATALOG_CB_CONSECUTIVE_FAILS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 { cbFailures = uint32(n) }
	}
	breaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:    "azure-client",
		Timeout: cbTimeout,
		ReadyToTrip: func(c gobreaker.Counts) bool { return c.ConsecutiveFailures >= cbFailures },
	})

	return &AzureClientAdapter{ service: svc, container: container, breaker: breaker }, nil
}

// DeleteBlob deletes a single blob from Azure storage
func (a *AzureClientAdapter) DeleteBlob(ctx context.Context, blobPath string) error {
	attemptTimeout := 3 * time.Second
	if v := os.Getenv("CATALOG_AZURE_TIMEOUT_MS"); v != "" {
		if d, err := time.ParseDuration(v + "ms"); err == nil { attemptTimeout = d }
	}
	retries := 2
	if v := os.Getenv("CATALOG_AZURE_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 { retries = n }
	}
	var last error
	backoff := 200 * time.Millisecond
	for i := 0; i <= retries; i++ {
		c, cancel := context.WithTimeout(ctx, attemptTimeout)
		_, err := a.breaker.Execute(func() (interface{}, error) {
			return a.service.DeleteBlob(c, a.container, blobPath, nil)
		})
		cancel()
		if err == nil { return nil }
		last = err
		if i < retries { time.Sleep(backoff); if backoff < 1500*time.Millisecond { backoff *= 2 } }
	}
	return last
}

// DeleteBlobsWithPrefix deletes all blobs with the given prefix from Azure storage
func (a *AzureClientAdapter) DeleteBlobsWithPrefix(ctx context.Context, prefix string) error {
	pager := a.service.NewListBlobsFlatPager(a.container, &azblob.ListBlobsFlatOptions{ Prefix: &prefix })
	for pager.More() {
		// Wrap each page retrieval with breaker
		pageAny, err := a.breaker.Execute(func() (interface{}, error) { return pager.NextPage(ctx) })
		if err != nil { return fmt.Errorf("failed to list blobs with prefix %s: %w", prefix, err) }
		page := pageAny.(azblob.ListBlobsFlatResponse)
		for _, b := range page.Segment.BlobItems {
			if b.Name != nil {
				if err := a.DeleteBlob(ctx, *b.Name); err != nil { return fmt.Errorf("failed to delete blob %s: %w", *b.Name, err) }
			}
		}
	}
	return nil
}

// BlobExists checks if a blob exists in Azure storage
func (a *AzureClientAdapter) BlobExists(ctx context.Context, blobPath string) (bool, error) {
	pager := a.service.NewListBlobsFlatPager(a.container, &azblob.ListBlobsFlatOptions{ Prefix: &blobPath })
	if pager.More() {
		pageAny, err := a.breaker.Execute(func() (interface{}, error) { return pager.NextPage(ctx) })
		if err != nil { return false, fmt.Errorf("failed to check blob existence: %w", err) }
		page := pageAny.(azblob.ListBlobsFlatResponse)
		for _, b := range page.Segment.BlobItems {
			if b.Name != nil && *b.Name == blobPath { return true, nil }
		}
	}
	return false, nil
}
