# Video Catalog API

A Go-based microservice for the StreamHive video platform that provides a REST API for video catalog management and consumes video lifecycle events.

## Features

- **REST API**: CRUD operations for video metadata
- **Event Processing**: Consumes both `video.uploaded` (seed) and `video.transcoded` (finalize) events from RabbitMQ
- **Database**: PostgreSQL with GORM
- **Search**: Title / description / tag search
- **Privacy Controls**: Public/private
- **Pagination**
- **Observability**: Zap + Prometheus
- **Cloud Native**: Docker & Kubernetes

## Event Flow
1. UploadService publishes `video.uploaded` to exchange `streamhive` (routing key `video.uploaded`).
2. TranscoderService consumes, transcodes, then publishes `video.transcoded` (routing key `video.transcoded`).
3. VideoCatalogService consumes both:
   - `video.uploaded`: create row (status=processing)
   - `video.transcoded`: update row with HLS URL + metadata (status=ready)

## API Endpoints

### Videos
- `GET /api/v1/videos` - List public videos
- `POST /api/v1/videos` - Manually register (requires existing `upload_id` from UploadService)
- `GET /api/v1/videos/:id` - Get by ID
- `PUT /api/v1/videos/:id` - Update
- `DELETE /api/v1/videos/:id` - Soft delete
- `GET /api/v1/videos/search?q=query` - Search

### User Videos
- `GET /api/v1/users/:userID/videos`

### System
- `GET /health`
- `GET /metrics`

## Create Video (manual)
Provide the `upload_id` returned by UploadService:
```bash
curl -X POST http://localhost:8080/api/v1/videos \
  -H "Content-Type: application/json" \
  -H "X-User-ID: user123" \
  -d '{
    "upload_id": "<existing-upload-id>",
    "title": "My Amazing Video",
    "description": "A description",
    "tags": ["tutorial", "golang"],
    "category": "education",
    "is_private": false
  }'
```

## Required Environment (added)
- `AMQP_UPLOAD_QUEUE` (default: video-catalog.video.uploaded)
- `AMQP_UPLOAD_ROUTING_KEY` (default: video.uploaded)

## Testing Event Flow Quickly
Publish a mock uploaded event:
```bash
rabbitmqadmin publish exchange=streamhive routing_key=video.uploaded payload='{"uploadId":"u1","userId":"user123","title":"Test","description":"Demo","tags":["demo"],"isPrivate":false,"category":"general","rawVideoPath":"raw/user123/u1.mp4"}'
```
Then publish transcoded:
```bash
rabbitmqadmin publish exchange=streamhive routing_key=video.transcoded payload='{"uploadId":"u1","userId":"user123","hls":{"masterUrl":"https://example/hls/user123/u1/master.m3u8"},"ready":true,"metadata":{"duration":10,"fileSize":1000,"width":1280,"height":720,"videoCodec":"h264","videoBitrate":500000,"audioCodec":"aac","audioBitrate":128000,"frameRate":30}}'
```

## Notes
If a `video.transcoded` arrives before `video.uploaded`, the service upserts by creating a placeholder row.
