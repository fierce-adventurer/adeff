package mq

import (
	"context"
	"encoding/json"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitClient struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
}

// AnalyzeEvent is the exact payload the Gateway will send to the Worker
type AnalyzeEvent struct {
	BookID   string `json:"book_id"`
	PdfS3Key string `json:"pdf_s3_key"`
}

func ConnectRabbitMQ(amqpURL string) (*RabbitClient, error) {
	// The amqp091-go driver automatically handles the TLS handshake for amqps:// URLs
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	// Declare the queue here so it is guaranteed to exist
	// before the Gateway tries to publish to it.

	_, err = ch.QueueDeclare(
		"queue.analyze", // name of the queue
		true,            // durable (survives RabbitMQ restarts)
		false,           // delete when unused
		false,           // exclusive
		false,           // no-wait
		nil,             // arguments
	)

	if err != nil {
		return nil, err
	}

	log.Println("Successfully connected to CloudAMQP")
	return &RabbitClient{
		Conn:    conn,
		Channel: ch,
	}, nil
}

func (rc *RabbitClient) PublishAnalyzeEvent(event AnalyzeEvent) error {
	// 5-second timeout so the Gateway doesn't hang forever if CloudAMQP glitches
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	err = rc.Channel.PublishWithContext(ctx,
		"",
		"queue.analyze", // routing key = queue name for default exchange
		false,           // mandatory
		false,           // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
	return err
}

func (rc *RabbitClient) ConsumeAnalyzeQueue() (<-chan amqp.Delivery, error) {
	// Tells RabbitMQ to send us messages from the queue we declared earlier
	msgs, err := rc.Channel.Consume(
		"queue.analyze", // queue
		"",              // consumer
		false,           // auto-ack (we'll ack manually after processing)
		false,           // exclusive
		false,           // no-local
		false,           // no-wait
		nil,             // args
	)
	return msgs, err
}
