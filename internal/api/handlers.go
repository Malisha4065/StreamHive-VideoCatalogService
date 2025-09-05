package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/streamhive/video-catalog-api/internal/models"
	"github.com/streamhive/video-catalog-api/internal/services"
)

// VideoHandler handles video-related HTTP requests
type VideoHandler struct {
	videoService *services.VideoService
	logger       *zap.SugaredLogger
}

// NewVideoHandler creates a new video handler
func NewVideoHandler(videoService *services.VideoService, logger *zap.SugaredLogger) *VideoHandler {
	return &VideoHandler{
		videoService: videoService,
		logger:       logger,
	}
}

// SetupRoutes sets up all API routes
func SetupRoutes(router *gin.Engine, videoService *services.VideoService, logger *zap.SugaredLogger) {
	handler := NewVideoHandler(videoService, logger)

	api := router.Group("/api/v1")
	{
		videos := api.Group("/videos")
		{
			videos.GET("", handler.ListVideos)
			videos.POST("", handler.CreateVideo)
			videos.GET("/:id", handler.GetVideo)
			videos.PUT("/:id", handler.UpdateVideo)
			videos.DELETE("/:id", handler.DeleteVideo)
			videos.GET("/search", handler.SearchVideos)
			videos.GET("/upload/:uploadId", handler.GetVideoByUploadID)
		}

		// User-specific routes
		users := api.Group("/users/:userID/videos")
		{
			users.GET("", handler.ListUserVideos)
		}
	}
}

// ListVideos handles GET /api/v1/videos
func (h *VideoHandler) ListVideos(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	response, err := h.videoService.ListVideos("", page, perPage, false)
	if err != nil {
		h.logger.Errorw("Failed to list videos", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list videos"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ListUserVideos handles GET /api/v1/users/:userID/videos
func (h *VideoHandler) ListUserVideos(c *gin.Context) {
	userID := c.Param("userID")
	requesterID := c.GetHeader("X-User-ID")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	// Include private only if caller is the owner
	includePrivate := requesterID != "" && requesterID == userID

	response, err := h.videoService.ListVideos(userID, page, perPage, includePrivate)
	if err != nil {
		h.logger.Errorw("Failed to list user videos", "error", err, "userID", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list videos"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// CreateVideo handles POST /api/v1/videos
func (h *VideoHandler) CreateVideo(c *gin.Context) {
	var req models.VideoCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UploadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "upload_id is required (obtain from UploadService)"})
		return
	}

	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID required"})
		return
	}

	video, err := h.videoService.CreateVideo(userID, &req)
	if err != nil {
		h.logger.Errorw("Failed to create video", "error", err, "userID", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create video"})
		return
	}

	c.JSON(http.StatusCreated, video)
}

// GetVideo handles GET /api/v1/videos/:id
func (h *VideoHandler) GetVideo(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	video, err := h.videoService.GetVideo(uint(id))
	if err != nil {
		if err.Error() == "video not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
			return
		}
		h.logger.Errorw("Failed to get video", "error", err, "videoID", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get video"})
		return
	}

	c.JSON(http.StatusOK, video)
}

// UpdateVideo handles PUT /api/v1/videos/:id
func (h *VideoHandler) UpdateVideo(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	var req models.VideoUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	video, err := h.videoService.UpdateVideo(uint(id), &req)
	if err != nil {
		if err.Error() == "video not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
			return
		}
		h.logger.Errorw("Failed to update video", "error", err, "videoID", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update video"})
		return
	}

	c.JSON(http.StatusOK, video)
}

// DeleteVideo handles DELETE /api/v1/videos/:id - permanently removes video and all files
func (h *VideoHandler) DeleteVideo(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	if err := h.videoService.DeleteVideo(uint(id)); err != nil {
		if err.Error() == "video not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
			return
		}
		h.logger.Errorw("Failed to delete video", "error", err, "videoID", id)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete video"})
		return
	}

	h.logger.Infow("Video permanently deleted", "videoID", id)
	c.JSON(http.StatusOK, gin.H{
		"message": "Video and all associated files have been permanently deleted",
		"video_id": id,
	})
}

// SearchVideos handles GET /api/v1/videos/search
func (h *VideoHandler) SearchVideos(c *gin.Context) {
	query := c.Query("q")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	response, err := h.videoService.SearchVideos(query, page, perPage)
	if err != nil {
		h.logger.Errorw("Failed to search videos", "error", err, "query", query)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search videos"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetVideoByUploadID handles GET /api/v1/videos/upload/:uploadId
func (h *VideoHandler) GetVideoByUploadID(c *gin.Context) {
	uploadID := c.Param("uploadId")
	if uploadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uploadId required"})
		return
	}
	video, err := h.videoService.GetVideoByUploadID(uploadID)
	if err != nil {
		if err.Error() == "video not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
			return
		}
		h.logger.Errorw("Failed to get video by uploadId", "error", err, "uploadId", uploadID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get video"})
		return
	}
	c.JSON(http.StatusOK, video)
}
