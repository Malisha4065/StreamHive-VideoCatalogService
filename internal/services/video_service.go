package services

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/streamhive/video-catalog-api/internal/models"
)

// VideoService handles video-related business logic
type VideoService struct {
	db            *gorm.DB
	logger        *zap.SugaredLogger
	deleteService *VideoDeleteService
}

// NewVideoService creates a new video service
func NewVideoService(db *gorm.DB, logger *zap.SugaredLogger) *VideoService {
	// Initialize Azure client for deletion operations
	azureClient, err := NewAzureClientAdapterFromEnv()
	if err != nil {
		logger.Warnw("Failed to initialize Azure client for video deletion", "error", err)
		// Continue without deletion service - deletion will be database-only
		return &VideoService{db: db, logger: logger, deleteService: nil}
	}

	deleteService := NewVideoDeleteService(db, logger, azureClient)
	return &VideoService{db: db, logger: logger, deleteService: deleteService}
}

// CreateVideo creates a new video record (manual creation path)
func (s *VideoService) CreateVideo(userID string, req *models.VideoCreateRequest) (*models.Video, error) {
	if req.UploadID == "" {
		return nil, fmt.Errorf("upload_id required")
	}

	video := &models.Video{
		UploadID:    req.UploadID,
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
		TagsList:    req.Tags,
		IsPrivate:   req.IsPrivate,
		Category:    req.Category,
		Status:      models.StatusUploaded,
	}

	if err := s.db.Create(video).Error; err != nil {
		s.logger.Errorw("Failed to create video", "error", err, "userID", userID, "uploadID", req.UploadID)
		return nil, fmt.Errorf("failed to create video: %w", err)
	}

	s.logger.Infow("Video created", "videoID", video.ID, "userID", userID, "uploadID", req.UploadID)
	return video, nil
}

// GetVideo retrieves a video by ID
func (s *VideoService) GetVideo(id uint) (*models.Video, error) {
	var video models.Video
	if err := s.db.First(&video, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("video not found")
		}
		s.logger.Errorw("Failed to get video", "error", err, "videoID", id)
		return nil, fmt.Errorf("failed to get video: %w", err)
	}
	return &video, nil
}

// GetVideoByUploadID retrieves a video by upload ID
func (s *VideoService) GetVideoByUploadID(uploadID string) (*models.Video, error) {
	var video models.Video
	if err := s.db.Where("upload_id = ?", uploadID).First(&video).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("video not found")
		}
		s.logger.Errorw("Failed to get video by upload ID", "error", err, "uploadID", uploadID)
		return nil, fmt.Errorf("failed to get video: %w", err)
	}
	return &video, nil
}

// UpdateVideo updates a video record
func (s *VideoService) UpdateVideo(id uint, req *models.VideoUpdateRequest) (*models.Video, error) {
	video, err := s.GetVideo(id)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	if req.Title != nil {
		video.Title = *req.Title
	}
	if req.Description != nil {
		video.Description = *req.Description
	}
	if req.Tags != nil {
		video.TagsList = req.Tags
	}
	if req.IsPrivate != nil {
		video.IsPrivate = *req.IsPrivate
	}
	if req.Category != nil {
		video.Category = *req.Category
	}

	if err := s.db.Save(video).Error; err != nil {
		s.logger.Errorw("Failed to update video", "error", err, "videoID", id)
		return nil, fmt.Errorf("failed to update video: %w", err)
	}

	s.logger.Infow("Video updated", "videoID", id)
	return video, nil
}

// DeleteVideo completely removes a video and all associated files
func (s *VideoService) DeleteVideo(id uint) error {
	// Use the delete service if available for complete cleanup
	if s.deleteService != nil {
		ctx := context.Background()
		if err := s.deleteService.DeleteVideoCompletely(ctx, id); err != nil {
			s.logger.Errorw("Failed to delete video completely", "error", err, "videoID", id)
			return err
		}
		return nil
	}

	// Fallback to database-only deletion if Azure client unavailable
	s.logger.Warnw("Azure client not available - performing database-only deletion", "videoID", id)
	if err := s.db.Delete(&models.Video{}, id).Error; err != nil {
		s.logger.Errorw("Failed to delete video from database", "error", err, "videoID", id)
		return fmt.Errorf("failed to delete video: %w", err)
	}
	s.logger.Infow("Video deleted from database only", "videoID", id)
	return nil
}

// ListVideos retrieves a paginated list of videos for a user
func (s *VideoService) ListVideos(userID string, page, perPage int, includePrivate bool) (*models.VideoListResponse, error) {
	var videos []models.Video
	var total int64
	query := s.db.Model(&models.Video{})
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if !includePrivate {
		query = query.Where("is_private = ?", false)
	}
	if err := query.Count(&total).Error; err != nil {
		s.logger.Errorw("Failed to count videos", "error", err, "userID", userID)
		return nil, fmt.Errorf("failed to count videos: %w", err)
	}
	offset := (page - 1) * perPage
	if err := query.Offset(offset).Limit(perPage).Order("created_at DESC").Find(&videos).Error; err != nil {
		s.logger.Errorw("Failed to list videos", "error", err, "userID", userID)
		return nil, fmt.Errorf("failed to list videos: %w", err)
	}
	totalPages := int((total + int64(perPage) - 1) / int64(perPage))
	return &models.VideoListResponse{Videos: videos, Total: total, Page: page, PerPage: perPage, TotalPages: totalPages}, nil
}

// SearchVideos searches for videos by title, description, or tags
func (s *VideoService) SearchVideos(query string, page, perPage int) (*models.VideoListResponse, error) {
	var videos []models.Video
	var total int64
	searchQuery := s.db.Model(&models.Video{}).Where("is_private = ?", false)
	if query != "" {
		pattern := "%" + query + "%"
		searchQuery = searchQuery.Where("title ILIKE ? OR description ILIKE ? OR ? = ANY(tags)", pattern, pattern, query)
	}
	if err := searchQuery.Count(&total).Error; err != nil {
		s.logger.Errorw("Failed to count search results", "error", err, "query", query)
		return nil, fmt.Errorf("failed to count search results: %w", err)
	}
	offset := (page - 1) * perPage
	if err := searchQuery.Offset(offset).Limit(perPage).Order("created_at DESC").Find(&videos).Error; err != nil {
		s.logger.Errorw("Failed to search videos", "error", err, "query", query)
		return nil, fmt.Errorf("failed to search videos: %w", err)
	}
	totalPages := int((total + int64(perPage) - 1) / int64(perPage))
	return &models.VideoListResponse{Videos: videos, Total: total, Page: page, PerPage: perPage, TotalPages: totalPages}, nil
}

// HandleUploadedEvent seeds catalog from upload event
func (s *VideoService) HandleUploadedEvent(event *models.UploadedEvent) error {
	if event.UploadID == "" || event.UserID == "" {
		return fmt.Errorf("invalid uploaded event")
	}

	var existing models.Video
	err := s.db.Where("upload_id = ?", event.UploadID).First(&existing).Error
	if err == nil {
		// Row already exists – possibly created from a prior transcoded event placeholder.
		updated := false
		// Only patch empty / default fields so we don't overwrite user edits.
		if existing.Username == "" && event.Username != "" {
			existing.Username = event.Username
			updated = true
		}
		if existing.Title == "Untitled Video" && event.Title != "" {
			existing.Title = event.Title
			updated = true
		}
		if existing.Description == "" && event.Description != "" {
			existing.Description = event.Description
			updated = true
		}
		if len(existing.TagsList) == 0 && len(event.Tags) > 0 {
			existing.TagsList = event.Tags
			updated = true
		}
		if existing.Category == "" && event.Category != "" {
			existing.Category = event.Category
			updated = true
		}
		if existing.OriginalFilename == "" && event.OriginalName != "" {
			existing.OriginalFilename = event.OriginalName
			updated = true
		}
		if existing.RawVideoPath == "" && event.RawVideoPath != "" {
			existing.RawVideoPath = event.RawVideoPath
			updated = true
		}
		// Always trust privacy flag if row had default false and upload says true.
		if !existing.IsPrivate && event.IsPrivate {
			existing.IsPrivate = true
			updated = true
		}
		if updated {
			if err := s.db.Save(&existing).Error; err != nil {
				return fmt.Errorf("patch existing video from upload event: %w", err)
			}
			s.logger.Infow("Patched existing video with upload metadata", "uploadID", event.UploadID, "videoID", existing.ID)
		}
		return nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("query existing: %w", err)
	}

	video := &models.Video{
		UploadID:         event.UploadID,
		UserID:           event.UserID,
		Username:         event.Username,
		Title:            nonEmpty(event.Title, "Untitled Video"),
		Description:      event.Description,
		TagsList:         event.Tags,
		IsPrivate:        event.IsPrivate,
		Category:         event.Category,
		OriginalFilename: event.OriginalName,
		RawVideoPath:     event.RawVideoPath,
		Status:           models.StatusProcessing,
	}

	if err := s.db.Create(video).Error; err != nil {
		s.logger.Errorw("Failed to create video from uploaded event", "error", err, "uploadID", event.UploadID)
		return fmt.Errorf("failed to create video: %w", err)
	}

	s.logger.Infow("Catalog seeded from upload event", "uploadID", event.UploadID, "videoID", video.ID)
	return nil
}

// HandleTranscodedEvent processes video.transcoded events
func (s *VideoService) HandleTranscodedEvent(event *models.TranscodedEvent) error {
	video, err := s.GetVideoByUploadID(event.UploadID)
	if err != nil {
		video = &models.Video{
			UploadID: event.UploadID,
			UserID:   event.UserID,
			Title:    nonEmpty(event.Title, "Untitled Video"),
			Status:   models.StatusProcessing,
		}
		if err := s.db.Create(video).Error; err != nil {
			s.logger.Errorw("Failed to create video from transcoded event", "error", err, "uploadID", event.UploadID)
			return fmt.Errorf("failed to create video: %w", err)
		}
	}

	// Backfill metadata if still empty / default
	updated := false
	if video.Title == "Untitled Video" && event.Title != "" {
		video.Title = event.Title
		updated = true
	}
	if video.Description == "" && event.Description != "" {
		video.Description = event.Description
		updated = true
	}
	if len(video.TagsList) == 0 && len(event.Tags) > 0 {
		video.TagsList = event.Tags
		updated = true
	}
	if video.Category == "" && event.Category != "" {
		video.Category = event.Category
		updated = true
	}
	if video.OriginalFilename == "" && event.OriginalFilename != "" {
		video.OriginalFilename = event.OriginalFilename
		updated = true
	}
	if video.RawVideoPath == "" && event.RawVideoPath != "" {
		video.RawVideoPath = event.RawVideoPath
		updated = true
	}
	if !video.IsPrivate && event.IsPrivate { // escalate privacy if needed
		video.IsPrivate = true
		updated = true
	}

	video.HLSMasterURL = event.HLS.MasterURL
	video.Status = models.StatusReady

	// Set thumbnail URL if provided
	if event.ThumbnailURL != "" {
		video.ThumbnailURL = event.ThumbnailURL
		updated = true
	}

	if event.Metadata != nil {
		video.Duration = event.Metadata.Duration
		video.FileSize = event.Metadata.FileSize
		video.Width = event.Metadata.Width
		video.Height = event.Metadata.Height
		video.VideoCodec = event.Metadata.VideoCodec
		video.VideoBitrate = event.Metadata.VideoBitrate
		video.AudioCodec = event.Metadata.AudioCodec
		video.AudioBitrate = event.Metadata.AudioBitrate
		video.FrameRate = event.Metadata.FrameRate
		updated = true
	}

	if err := s.db.Save(video).Error; err != nil {
		s.logger.Errorw("Failed to update video from transcoded event", "error", err, "uploadID", event.UploadID)
		return fmt.Errorf("failed to update video: %w", err)
	}

	if updated {
		s.logger.Infow("Video updated from transcoded event (metadata backfilled)", "uploadID", event.UploadID, "videoID", video.ID)
	} else {
		s.logger.Infow("Video status updated from transcoded event", "uploadID", event.UploadID, "videoID", video.ID)
	}
	return nil
}

func nonEmpty(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
