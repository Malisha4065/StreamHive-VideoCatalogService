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
	commentSvc   *services.CommentService
	logger       *zap.SugaredLogger
}

// NewVideoHandler creates a new video handler
func NewVideoHandler(videoService *services.VideoService, commentSvc *services.CommentService, logger *zap.SugaredLogger) *VideoHandler {
	return &VideoHandler{
		videoService: videoService,
	commentSvc:   commentSvc,
		logger:       logger,
	}
}

// SetupRoutes sets up all API routes
func SetupRoutes(router *gin.Engine, videoService *services.VideoService, logger *zap.SugaredLogger) {
	commentSvc := services.NewCommentService(videoService.DB(), logger)
	handler := NewVideoHandler(videoService, commentSvc, logger)

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
			// Comments on a video
			videos.GET("/:id/comments", handler.ListComments)
			videos.POST("/:id/comments", handler.AddComment)
		}

		// User-specific routes
		users := api.Group("/users/:userID/videos")
		{
			users.GET("", handler.ListUserVideos)
		}

	// Comment management
	api.DELETE("/comments/:commentID", handler.DeleteComment)
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

// ListComments handles GET /api/v1/videos/:id/comments
func (h *VideoHandler) ListComments(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"}); return }
	video, err := h.videoService.GetVideo(uint(id))
	if err != nil { c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"}); return }
	requester := c.GetHeader("X-User-ID")
	// Enforce privacy: if private, only owner sees comments
	if video.IsPrivate && video.UserID != requester {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"}); return
	}
	comments, err := h.commentSvc.ListComments(uint(id), !video.IsPrivate, requester)
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list comments"}); return }
	c.JSON(http.StatusOK, gin.H{"comments": comments})
}

// AddComment handles POST /api/v1/videos/:id/comments
func (h *VideoHandler) AddComment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"}); return }
	requester := c.GetHeader("X-User-ID")
	if requester == "" { c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID required"}); return }
	video, err := h.videoService.GetVideo(uint(id))
	if err != nil { c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"}); return }
	// If private, only owner can comment (policy; adjust as needed)
	if video.IsPrivate && video.UserID != requester {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"}); return
	}
	var req models.CommentCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
	cmt, err := h.commentSvc.AddComment(uint(id), requester, req.Content)
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add comment"}); return }
	c.JSON(http.StatusCreated, cmt)
}

// DeleteComment handles DELETE /api/v1/comments/:commentID
func (h *VideoHandler) DeleteComment(c *gin.Context) {
	cid, err := strconv.ParseUint(c.Param("commentID"), 10, 32)
	if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid comment ID"}); return }
	requester := c.GetHeader("X-User-ID")
	if requester == "" { c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID required"}); return }
	// Load comment and video to determine permission: author or video owner can delete
	var comment models.Comment
	if err := h.videoService.DB().First(&comment, uint(cid)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comment not found"}); return
	}
	video, err := h.videoService.GetVideo(comment.VideoID)
	if err != nil { c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"}); return }
	isOwnerOrAuthor := (comment.UserID == requester) || (video.UserID == requester)
	if err := h.commentSvc.DeleteComment(uint(cid), requester, isOwnerOrAuthor); err != nil {
		if err.Error() == "forbidden" { c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"}); return }
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete comment"}); return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
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
