package models

import (
	"encoding/json"
	"strings"
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
	Tags        string      `json:"-" gorm:"type:text[]"`
	TagsList    []string    `json:"tags" gorm:"-"`
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

// Comment represents a comment on a video
type Comment struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	VideoID   uint           `json:"video_id" gorm:"index;not null"`
	UserID    string         `json:"user_id" gorm:"index;not null"`
	Content   string         `json:"content" gorm:"type:text;not null"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

type CommentCreateRequest struct {
	Content string `json:"content" binding:"required,min=1,max=2000"`
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
// Now optionally carries original metadata so catalog can backfill if upload event missed.
type TranscodedEvent struct {
	UploadID         string         `json:"uploadId"`
	UserID           string         `json:"userId"`
	Title            string         `json:"title,omitempty"`
	Description      string         `json:"description,omitempty"`
	Tags             []string       `json:"tags,omitempty"`
	Category         string         `json:"category,omitempty"`
	IsPrivate        bool           `json:"isPrivate,omitempty"`
	OriginalFilename string         `json:"originalFilename,omitempty"`
	RawVideoPath     string         `json:"rawVideoPath,omitempty"`
	HLS              HLSInfo        `json:"hls"`
	ThumbnailURL     string         `json:"thumbnailUrl,omitempty"`
	Ready            bool           `json:"ready"`
	Metadata         *VideoMetadata `json:"metadata,omitempty"`
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

// UnmarshalJSON implements custom unmarshaling for UploadedEvent to handle tags
func (e *UploadedEvent) UnmarshalJSON(data []byte) error {
	type Alias UploadedEvent
	aux := &struct {
		Tags interface{} `json:"tags"`
		*Alias
	}{
		Alias: (*Alias)(e),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Handle tags field - it could be a string or array
	switch v := aux.Tags.(type) {
	case string:
		if v == "" {
			e.Tags = []string{}
		} else {
			e.Tags = strings.Split(v, ",")
			for i, tag := range e.Tags {
				e.Tags[i] = strings.TrimSpace(tag)
			}
		}
	case []interface{}:
		e.Tags = make([]string, len(v))
		for i, tag := range v {
			if str, ok := tag.(string); ok {
				e.Tags[i] = strings.TrimSpace(str)
			}
		}
	case []string:
		e.Tags = v
	case nil:
		e.Tags = []string{}
	default:
		e.Tags = []string{}
	}

	return nil
}

// UnmarshalJSON implements custom unmarshaling for TranscodedEvent to handle tags
func (e *TranscodedEvent) UnmarshalJSON(data []byte) error {
	type Alias TranscodedEvent
	aux := &struct {
		Tags interface{} `json:"tags"`
		*Alias
	}{
		Alias: (*Alias)(e),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Handle tags field - it could be a string or array
	switch v := aux.Tags.(type) {
	case string:
		if v == "" {
			e.Tags = []string{}
		} else {
			e.Tags = strings.Split(v, ",")
			for i, tag := range e.Tags {
				e.Tags[i] = strings.TrimSpace(tag)
			}
		}
	case []interface{}:
		e.Tags = make([]string, len(v))
		for i, tag := range v {
			if str, ok := tag.(string); ok {
				e.Tags[i] = strings.TrimSpace(str)
			}
		}
	case []string:
		e.Tags = v
	case nil:
		e.Tags = []string{}
	default:
		e.Tags = []string{}
	}

	return nil
}

// MarshalJSON custom marshals the UploadedEvent to handle tags properly
func (e *UploadedEvent) MarshalJSON() ([]byte, error) {
	type Alias UploadedEvent
	aux := &struct {
		Tags json.RawMessage `json:"tags"`
		*Alias
	}{
		Alias: (*Alias)(e),
	}

	// Marshal the tags field as a JSON array
	tagsData, err := json.Marshal(e.Tags)
	if err != nil {
		return nil, err
	}
	aux.Tags = tagsData

	return json.Marshal(aux)
}

// SanitizeTags sanitizes the tags by trimming spaces and removing empty values
func (e *UploadedEvent) SanitizeTags() {
	var sanitizedTags []string
	for _, tag := range e.Tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			sanitizedTags = append(sanitizedTags, tag)
		}
	}
	e.Tags = sanitizedTags
}

// BeforeCreate hook to convert TagsList to Tags before database insert
func (v *Video) BeforeCreate(tx *gorm.DB) error {
	v.Tags = convertSliceToPostgresArray(v.TagsList)
	return nil
}

// BeforeUpdate hook to convert TagsList to Tags before database update
func (v *Video) BeforeUpdate(tx *gorm.DB) error {
	v.Tags = convertSliceToPostgresArray(v.TagsList)
	return nil
}

// AfterFind hook to convert Tags to TagsList after database query
func (v *Video) AfterFind(tx *gorm.DB) error {
	v.TagsList = convertPostgresArrayToSlice(v.Tags)
	return nil
}

// MarshalJSON implements custom JSON marshaling for Video
func (v Video) MarshalJSON() ([]byte, error) {
	type Alias Video
	aux := &struct {
		Tags []string `json:"tags"`
		*Alias
	}{
		Tags:  v.TagsList,
		Alias: (*Alias)(&v),
	}
	// Remove the TagsList field from JSON output by setting it to nil in the alias
	aux.Alias.TagsList = nil
	return json.Marshal(aux)
}

// Helper function to convert Go slice to PostgreSQL array string
func convertSliceToPostgresArray(slice []string) string {
	if len(slice) == 0 {
		return "{}"
	}

	// Escape quotes and build array string
	var escaped []string
	for _, item := range slice {
		// Escape quotes by doubling them
		escaped = append(escaped, `"`+strings.ReplaceAll(item, `"`, `""`)+`"`)
	}

	return "{" + strings.Join(escaped, ",") + "}"
}

// Helper function to convert PostgreSQL array string to Go slice
func convertPostgresArrayToSlice(pgArray string) []string {
	if pgArray == "" || pgArray == "{}" {
		return []string{}
	}

	// Remove braces and split by comma
	trimmed := strings.Trim(pgArray, "{}")
	if trimmed == "" {
		return []string{}
	}

	parts := strings.Split(trimmed, ",")
	var result []string

	for _, part := range parts {
		// Remove quotes and unescape
		cleaned := strings.Trim(part, `"`)
		cleaned = strings.ReplaceAll(cleaned, `""`, `"`)
		result = append(result, cleaned)
	}

	return result
}
