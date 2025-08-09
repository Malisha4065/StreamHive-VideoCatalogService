package queue

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"github.com/streamhive/video-catalog-api/internal/models"
	"github.com/streamhive/video-catalog-api/internal/services"
)

// Consumer represents a RabbitMQ consumer
type Consumer struct {
	conn    *amqp091.Connection
	channel *amqp091.Channel
	logger  *zap.SugaredLogger
	// routing keys
	uploadedRoutingKey   string
	transcodedRoutingKey string
}

// NewConsumer creates a new RabbitMQ consumer
func NewConsumer(logger *zap.SugaredLogger) (*Consumer, error) {
	amqpURL := getEnv("AMQP_URL", "amqp://guest:guest@localhost:5672/")

	conn, err := amqp091.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	c := &Consumer{
		conn:                 conn,
		channel:              channel,
		logger:               logger,
		uploadedRoutingKey:   getEnv("AMQP_UPLOAD_ROUTING_KEY", "video.uploaded"),
		transcodedRoutingKey: getEnv("AMQP_ROUTING_KEY", "video.transcoded"),
	}

	if err := c.setupQueues(); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// setupQueues declares exchange and binds two queues (uploaded & transcoded)
func (c *Consumer) setupQueues() error {
	exchangeName := getEnv("AMQP_EXCHANGE", "streamhive")
	transcodedQueue := getEnv("AMQP_QUEUE", "video-catalog.video.transcoded")
	uploadedQueue := getEnv("AMQP_UPLOAD_QUEUE", "video-catalog.video.uploaded")

	if err := c.channel.ExchangeDeclare(exchangeName, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}
	if _, err := c.channel.QueueDeclare(transcodedQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare transcoded queue: %w", err)
	}
	if _, err := c.channel.QueueDeclare(uploadedQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare uploaded queue: %w", err)
	}
	if err := c.channel.QueueBind(transcodedQueue, c.transcodedRoutingKey, exchangeName, false, nil); err != nil {
		return fmt.Errorf("bind transcoded queue: %w", err)
	}
	if err := c.channel.QueueBind(uploadedQueue, c.uploadedRoutingKey, exchangeName, false, nil); err != nil {
		return fmt.Errorf("bind uploaded queue: %w", err)
	}

	c.logger.Infow("Queue setup completed", "exchange", exchangeName, "transcodedQueue", transcodedQueue, "uploadedQueue", uploadedQueue, "uploadedRoutingKey", c.uploadedRoutingKey, "transcodedRoutingKey", c.transcodedRoutingKey)
	return nil
}

// StartConsuming starts consuming both uploaded & transcoded queues
func (c *Consumer) StartConsuming(videoService *services.VideoService) error {
	transcodedQueue := getEnv("AMQP_QUEUE", "video-catalog.video.transcoded")
	uploadedQueue := getEnv("AMQP_UPLOAD_QUEUE", "video-catalog.video.uploaded")

	if err := c.channel.Qos(1, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	transcodedMsgs, err := c.channel.Consume(transcodedQueue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume transcoded: %w", err)
	}
	uploadedMsgs, err := c.channel.Consume(uploadedQueue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume uploaded: %w", err)
	}

	c.logger.Infow("Started consuming messages", "transcodedQueue", transcodedQueue, "uploadedQueue", uploadedQueue)

	// Merge channels using goroutines
	done := make(chan error, 2)
	go c.consumeLoop(uploadedMsgs, videoService, true, done)
	go c.consumeLoop(transcodedMsgs, videoService, false, done)
	// Block until one loop ends (on channel close)
	return <-done
}

func (c *Consumer) consumeLoop(msgs <-chan amqp091.Delivery, videoService *services.VideoService, isUploaded bool, done chan<- error) {
	for msg := range msgs {
		var err error
		if isUploaded {
			err = c.handleUploaded(msg, videoService)
		} else {
			err = c.handleTranscoded(msg, videoService)
		}
		if err != nil {
			c.logger.Errorw("Failed to handle message", "error", err, "uploaded", isUploaded)
			msg.Nack(false, false)
			continue
		}
		msg.Ack(false)
	}
	done <- fmt.Errorf("channel closed")
}

func (c *Consumer) handleUploaded(msg amqp091.Delivery, videoService *services.VideoService) error {
	c.logger.Debugw("Received upload event", "routingKey", msg.RoutingKey)
	var event models.UploadedEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		return fmt.Errorf("unmarshal uploaded: %w", err)
	}
	return videoService.HandleUploadedEvent(&event)
}

func (c *Consumer) handleTranscoded(msg amqp091.Delivery, videoService *services.VideoService) error {
	c.logger.Debugw("Received transcoded event", "routingKey", msg.RoutingKey)
	var event models.TranscodedEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		return fmt.Errorf("unmarshal transcoded: %w", err)
	}
	return videoService.HandleTranscodedEvent(&event)
}

// Close closes the consumer connection
func (c *Consumer) Close() {
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
