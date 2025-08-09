<!-- Use this file to provide workspace-specific custom instructions to Copilot. For more details, visit https://code.visualstudio.com/docs/copilot/copilot-customization#_use-a-githubcopilotinstructionsmd-file -->

# Video Catalog API Service Instructions

This is a Go-based microservice for the StreamHive video platform that serves as the Video Catalog API.

## Architecture Guidelines
- Follow clean architecture principles with clear separation between API, business logic, and data layers
- Use dependency injection for testability
- Implement proper error handling and logging
- Use structured logging with contextual information
- Follow Go best practices and idiomatic code patterns

## Key Components
- **REST API**: Gin web framework for HTTP endpoints
- **Database**: PostgreSQL with GORM ORM
- **Message Queue**: RabbitMQ consumer for video.transcoded events
- **Metrics**: Prometheus metrics for observability
- **Configuration**: Environment-based configuration

## API Design
- RESTful endpoints following OpenAPI/Swagger specifications
- Proper HTTP status codes and error responses
- Request validation and sanitization
- Pagination for list endpoints
- Authentication/authorization ready structure

## Database Design
- Use GORM for ORM operations
- Include proper database migrations
- Follow database naming conventions
- Implement soft deletes where appropriate
- Add appropriate indexes for performance

## Event Handling
- Consume video.transcoded events from RabbitMQ
- Implement retry logic and dead letter queue handling
- Ensure idempotent event processing
- Add event validation and error handling

## Testing Strategy
- Unit tests for business logic
- Integration tests for database operations
- API endpoint testing
- Mock external dependencies for isolated testing
