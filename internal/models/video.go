package models

import (
	"time"

	"gorm.io/gorm"
)

// Video represents a video in the catalog
type Video struct {
	ID          uint        `json:"id" gorm:"primarykey"`
	UploadID    string      `json:"upload_id" gorm:"uniqueIndex;not null"`
	UserID      string      `json:"user_id" gorm:"index;not null"`
	Username    string      `json:"username"`
	Title       string      `json:"title" gorm:"not null"`
	Description string      `json:"description"`
	Tags        []string    `json:"tags" gorm:"type:text[]"`
	IsPrivate   bool        `json:"is_private" gorm:"default:false"`
	Category    string      `json:"category"`
	Status      VideoStatus `json:"status" gorm:"default:'uploaded'"`

	// File information
	OriginalFilename string `json:"original_filename"`
	RawVideoPath     string `json:"raw_video_path"`
	HLSMasterURL     string `json:"hls_master_url"`
	ThumbnailURL     string `json:"thumbnail_url"`

	// Video metadata
	Duration     float64 `json:"duration"`
	FileSize     int64   `json:"file_size"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	VideoCodec   string  `json:"video_codec"`
	VideoBitrate int     `json:"video_bitrate"`
	AudioCodec   string  `json:"audio_codec"`
	AudioBitrate int     `json:"audio_bitrate"`
	FrameRate    float64 `json:"frame_rate"`

	// Timestamps
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// VideoStatus represents the processing status of a video
type VideoStatus string

const (
	StatusUploaded   VideoStatus = "uploaded"
	StatusProcessing VideoStatus = "processing"
	StatusReady      VideoStatus = "ready"
	StatusFailed     VideoStatus = "failed"
)

// VideoCreateRequest represents the request payload for creating a video
// Now requires an upload_id so that catalog rows map to upload/transcode events
// Clients should first upload via UploadService to obtain this ID.
type VideoCreateRequest struct {
	UploadID    string   `json:"upload_id" binding:"required"`
	Title       string   `json:"title" binding:"required"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	IsPrivate   bool     `json:"is_private"`
	Category    string   `json:"category"`
}

// VideoUpdateRequest represents the request payload for updating a video
type VideoUpdateRequest struct {
	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	IsPrivate   *bool    `json:"is_private,omitempty"`
	Category    *string  `json:"category,omitempty"`
}

// VideoListResponse represents the response for listing videos
type VideoListResponse struct {
	Videos     []Video `json:"videos"`
	Total      int64   `json:"total"`
	Page       int     `json:"page"`
	PerPage    int     `json:"per_page"`
	TotalPages int     `json:"total_pages"`
}

// TranscodedEvent represents the event received when a video is transcoded
type TranscodedEvent struct {
	UploadID string         `json:"uploadId"`
	UserID   string         `json:"userId"`
	HLS      HLSInfo        `json:"hls"`
	Ready    bool           `json:"ready"`
	Metadata *VideoMetadata `json:"metadata,omitempty"`
}

// UploadedEvent represents the initial upload event published by UploadService
type UploadedEvent struct {
	UploadID      string   `json:"uploadId"`
	UserID        string   `json:"userId"`
	Username      string   `json:"username"`
	OriginalName  string   `json:"originalFilename"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Tags          []string `json:"tags"`
	IsPrivate     bool     `json:"isPrivate"`
	Category      string   `json:"category"`
	RawVideoPath  string   `json:"rawVideoPath"`
	ContainerName string   `json:"containerName"`
	BlobURL       string   `json:"blobUrl"`
}

// HLSInfo contains HLS-related information
type HLSInfo struct {
	MasterURL string `json:"masterUrl"`
}

// VideoMetadata contains video file metadata
type VideoMetadata struct {
	Duration     float64 `json:"duration"`
	FileSize     int64   `json:"fileSize"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	VideoCodec   string  `json:"videoCodec"`
	VideoBitrate int     `json:"videoBitrate"`
	AudioCodec   string  `json:"audioCodec"`
	AudioBitrate int     `json:"audioBitrate"`
	FrameRate    float64 `json:"frameRate"`
}
