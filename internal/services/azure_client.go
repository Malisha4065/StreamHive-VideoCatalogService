package services

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
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

	return &AzureClientAdapter{
		service:   svc,
		container: container,
	}, nil
}

// DeleteBlob deletes a single blob from Azure storage
func (a *AzureClientAdapter) DeleteBlob(ctx context.Context, blobPath string) error {
	_, err := a.service.DeleteBlob(ctx, a.container, blobPath, nil)
	return err
}

// DeleteBlobsWithPrefix deletes all blobs with the given prefix from Azure storage
func (a *AzureClientAdapter) DeleteBlobsWithPrefix(ctx context.Context, prefix string) error {
	pager := a.service.NewListBlobsFlatPager(a.container, &azblob.ListBlobsFlatOptions{
		Prefix: &prefix,
	})

	deletedCount := 0
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list blobs with prefix %s: %w", prefix, err)
		}

		for _, blob := range page.Segment.BlobItems {
			if blob.Name != nil {
				if err := a.DeleteBlob(ctx, *blob.Name); err != nil {
					return fmt.Errorf("failed to delete blob %s: %w", *blob.Name, err)
				}
				deletedCount++
			}
		}
	}

	return nil
}

// BlobExists checks if a blob exists in Azure storage
func (a *AzureClientAdapter) BlobExists(ctx context.Context, blobPath string) (bool, error) {
	pager := a.service.NewListBlobsFlatPager(a.container, &azblob.ListBlobsFlatOptions{
		Prefix: &blobPath,
	})

	if pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to check blob existence: %w", err)
		}

		for _, blob := range page.Segment.BlobItems {
			if blob.Name != nil && *blob.Name == blobPath {
				return true, nil
			}
		}
	}

	return false, nil
}
