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

func (s *CommentService) AddComment(videoID uint, userID, content string) (*models.Comment, error) {
    // Ensure video exists and visibility allows commenting (basic existence check here)
    var v models.Video
    if err := s.db.First(&v, videoID).Error; err != nil {
        if err == gorm.ErrRecordNotFound {
            return nil, fmt.Errorf("video not found")
        }
        return nil, fmt.Errorf("lookup video: %w", err)
    }
    c := &models.Comment{VideoID: videoID, UserID: userID, Content: content}
    if err := s.db.Create(c).Error; err != nil {
        s.logger.Errorw("create comment", "err", err)
        return nil, fmt.Errorf("failed to create comment: %w", err)
    }
    return c, nil
}

func (s *CommentService) ListComments(videoID uint, includePrivate bool, requesterID string) ([]models.Comment, error) {
    // Visibility policy enforced by caller (handler) before calling this
    var out []models.Comment
    if err := s.db.Where("video_id = ?", videoID).Order("created_at ASC").Find(&out).Error; err != nil {
        return nil, fmt.Errorf("list comments: %w", err)
    }
    return out, nil
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
