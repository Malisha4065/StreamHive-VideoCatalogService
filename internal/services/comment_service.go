package services

import (
    "fmt"

    "go.uber.org/zap"
    "gorm.io/gorm"

    "github.com/streamhive/video-catalog-api/internal/models"
)

type CommentService struct {
    db     *gorm.DB
    logger *zap.SugaredLogger
}

func NewCommentService(db *gorm.DB, logger *zap.SugaredLogger) *CommentService {
    return &CommentService{db: db, logger: logger}
}

func (s *CommentService) AddComment(videoID uint, userID, username, content string) (*models.Comment, error) {
    // Ensure video exists and visibility allows commenting (basic existence check here)
    var v models.Video
    if err := s.db.First(&v, videoID).Error; err != nil {
        if err == gorm.ErrRecordNotFound {
            return nil, fmt.Errorf("video not found")
        }
        return nil, fmt.Errorf("lookup video: %w", err)
    }
    c := &models.Comment{VideoID: videoID, UserID: userID, Username: username, Content: content}
    if err := s.db.Create(c).Error; err != nil {
        s.logger.Errorw("create comment", "err", err)
        return nil, fmt.Errorf("failed to create comment: %w", err)
    }
    return c, nil
}

func (s *CommentService) ListComments(videoID uint, page, perPage int) ([]models.Comment, int64, error) {
    // Pagination with newest first
    if page < 1 { page = 1 }
    if perPage < 1 || perPage > 100 { perPage = 20 }

    var total int64
    if err := s.db.Model(&models.Comment{}).Where("video_id = ?", videoID).Count(&total).Error; err != nil {
        return nil, 0, fmt.Errorf("count comments: %w", err)
    }

    var out []models.Comment
    if err := s.db.Where("video_id = ?", videoID).
        Order("created_at DESC").
        Limit(perPage).
        Offset((page-1)*perPage).
        Find(&out).Error; err != nil {
        return nil, 0, fmt.Errorf("list comments: %w", err)
    }
    return out, total, nil
}

func (s *CommentService) DeleteComment(commentID uint, requesterID string, isOwnerOrAuthor bool) error {
    if !isOwnerOrAuthor {
        return fmt.Errorf("forbidden")
    }
    if err := s.db.Delete(&models.Comment{}, commentID).Error; err != nil {
        return fmt.Errorf("delete comment: %w", err)
    }
    return nil
}
