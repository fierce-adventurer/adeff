package synthesizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"

	"adeff/internal/database"
	"adeff/internal/models"
	"adeff/pkg/mq"
	"adeff/pkg/storage"
	"adeff/pkg/tts"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Worker struct {
	Rabbit *mq.RabbitClient
}

func (w *Worker) Start() error {
	msgs, err := w.Rabbit.ConsumeSynthesizeQueue()
	if err != nil {
		return err
	}

	log.Println("Synthesizer Worker listening for audio events...")

	go func() {
		for d := range msgs {
			w.processMessage(d)
		}
	}()

	select {}
}

func (w *Worker) processMessage(d amqp.Delivery) {
	var event mq.SynthesizeEvent
	if err := json.Unmarshal(d.Body, &event); err != nil {
		log.Printf("Failed to decode: %v", err)
		d.Ack(false)
		return
	}

	log.Printf("▶STARTING AUDIO SYNTHESIS: Book %s", event.BookID)

	//Update Status
	database.DB.Model(&models.Book{}).Where("id = ?", event.BookID).Update("saga_status", "SYNTHESIZING")

	// Generate Audio via Sarvam
	log.Printf("Requesting %s audio from Sarvam...", event.Language)
	audioBytes, err := tts.GenerateAudio(event.Text, event.Language)
	if err != nil {
		log.Printf("Sarvam API FAILED: %v", err)
		database.DB.Model(&models.Book{}).Where("id = ?", event.BookID).Update("saga_status", "ERROR_AUDIO")
		d.Ack(false) // Don't requeue API errors automatically to save credits
		return
	}
	log.Printf("Audio generated! Size: %d bytes", len(audioBytes))

	// Upload to MinIO
	audioS3Key := fmt.Sprintf("audio-books/%s.wav", uuid.New().String())
	audioReader := bytes.NewReader(audioBytes) // Convert bytes to io.Reader for MinIO

	err = storage.UploadStream(context.Background(), audioS3Key, audioReader, int64(len(audioBytes)), "audio/wav")
	if err != nil {
		log.Printf("MinIO Upload FAILED: %v", err)
		database.DB.Model(&models.Book{}).Where("id = ?", event.BookID).Update("saga_status", "ERROR_STORAGE")
		d.Nack(false, true) // Requeue on network failure
		return
	}

	// Final DB Update (SAGA COMPLETED!)
	database.DB.Model(&models.Book{}).Where("id = ?", event.BookID).Updates(map[string]interface{}{
		"saga_status":  "COMPLETED",
		"audio_s3_key": audioS3Key,
	})
	log.Printf("SAGA COMPLETE: Book %s is ready for listening! File: %s", event.BookID, audioS3Key)

	d.Ack(false)
}
