package services

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/streamhive/video-catalog-api/internal/models"
)

// VideoDeleteService handles video deletion including storage cleanup
type VideoDeleteService struct {
	db     *gorm.DB
	logger *zap.SugaredLogger
	azure  AzureStorageClient
}

// AzureStorageClient interface for Azure operations needed for deletion
type AzureStorageClient interface {
	DeleteBlob(ctx context.Context, blobPath string) error
	DeleteBlobsWithPrefix(ctx context.Context, prefix string) error
	BlobExists(ctx context.Context, blobPath string) (bool, error)
}

// NewVideoDeleteService creates a new video delete service
func NewVideoDeleteService(db *gorm.DB, logger *zap.SugaredLogger, azure AzureStorageClient) *VideoDeleteService {
	return &VideoDeleteService{
		db:     db,
		logger: logger,
		azure:  azure,
	}
}

// DeleteVideoCompletely removes a video and all associated files from database and storage
func (s *VideoDeleteService) DeleteVideoCompletely(ctx context.Context, videoID uint) error {
	// First get the video to extract all file paths
	var video models.Video
	if err := s.db.First(&video, videoID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("video not found")
		}
		s.logger.Errorw("Failed to get video for deletion", "error", err, "videoID", videoID)
		return fmt.Errorf("failed to get video: %w", err)
	}

	s.logger.Infow("Starting complete video deletion",
		"videoID", videoID,
		"uploadID", video.UploadID,
		"userID", video.UserID,
		"title", video.Title)

	// Collect all storage paths to delete
	var pathsToDelete []string
	var prefixesToDelete []string

	// 1. Raw video file
	if video.RawVideoPath != "" {
		pathsToDelete = append(pathsToDelete, video.RawVideoPath)
		s.logger.Infow("Will delete raw video", "path", video.RawVideoPath)
	}

	// 2. HLS files (all renditions, segments, and master playlist)
	if video.HLSMasterURL != "" {
		hlsPrefix := s.extractHLSPrefix(video.HLSMasterURL, video.UserID, video.UploadID)
		if hlsPrefix != "" {
			prefixesToDelete = append(prefixesToDelete, hlsPrefix)
			s.logger.Infow("Will delete HLS files", "prefix", hlsPrefix)
		}
	}

	// 3. Thumbnail
	thumbnailPath := fmt.Sprintf("thumbnails/%s/%s.jpg", video.UserID, video.UploadID)
	pathsToDelete = append(pathsToDelete, thumbnailPath)
	s.logger.Infow("Will delete thumbnail", "path", thumbnailPath)

	// 4. Any other potential files (future-proofing)
	otherPrefix := fmt.Sprintf("videos/%s/%s", video.UserID, video.UploadID)
	prefixesToDelete = append(prefixesToDelete, otherPrefix)

	// Delete from storage first (easier to retry if DB deletion fails)
	deletedFiles := 0
	deletedPrefixes := 0

	// Delete individual files
	for _, path := range pathsToDelete {
		if err := s.deleteFileIfExists(ctx, path); err != nil {
			s.logger.Warnw("Failed to delete file (continuing)", "error", err, "path", path)
		} else {
			deletedFiles++
		}
	}

	// Delete by prefix (for HLS folders)
	for _, prefix := range prefixesToDelete {
		if err := s.azure.DeleteBlobsWithPrefix(ctx, prefix); err != nil {
			s.logger.Warnw("Failed to delete files with prefix (continuing)", "error", err, "prefix", prefix)
		} else {
			deletedPrefixes++
		}
	}

	s.logger.Infow("Storage cleanup completed",
		"deletedFiles", deletedFiles,
		"deletedPrefixes", deletedPrefixes,
		"videoID", videoID)

	// Now delete from database (hard delete, not soft delete)
	if err := s.db.Unscoped().Delete(&video).Error; err != nil {
		s.logger.Errorw("Failed to delete video from database", "error", err, "videoID", videoID)
		return fmt.Errorf("failed to delete video from database: %w", err)
	}

	s.logger.Infow("Video completely deleted",
		"videoID", videoID,
		"uploadID", video.UploadID,
		"title", video.Title)

	return nil
}

// deleteFileIfExists deletes a file if it exists, ignoring not-found errors
func (s *VideoDeleteService) deleteFileIfExists(ctx context.Context, path string) error {
	exists, err := s.azure.BlobExists(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to check if file exists: %w", err)
	}

	if !exists {
		s.logger.Debugw("File doesn't exist, skipping", "path", path)
		return nil
	}

	if err := s.azure.DeleteBlob(ctx, path); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	s.logger.Debugw("File deleted", "path", path)
	return nil
}

// extractHLSPrefix extracts the HLS storage prefix from the master URL
func (s *VideoDeleteService) extractHLSPrefix(masterURL, userID, uploadID string) string {
	// Expected format: https://{account}.blob.core.windows.net/{container}/hls/{userID}/{uploadID}/master.m3u8
	// We want to extract: hls/{userID}/{uploadID}

	if masterURL == "" {
		return ""
	}

	// Try to extract from URL
	parts := strings.Split(masterURL, "/")
	for i, part := range parts {
		if part == "hls" && i+2 < len(parts) {
			// Found hls/{userID}/{uploadID}/master.m3u8
			return filepath.Join("hls", parts[i+1], parts[i+2])
		}
	}

	// Fallback: construct from known user and upload IDs
	return fmt.Sprintf("hls/%s/%s", userID, uploadID)
}
