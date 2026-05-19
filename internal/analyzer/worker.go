package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"

	"adeff/internal/database"
	"adeff/internal/models"
	"adeff/pkg/llm"
	"adeff/pkg/mq"
	"adeff/pkg/storage"

	"github.com/ledongthuc/pdf"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Worker struct {
	Rabbit *mq.RabbitClient
}

func (w *Worker) Start() error {
	msgs, err := w.Rabbit.ConsumeAnalyzeQueue()
	if err != nil {
		return err
	}

	log.Println("Analyzer Worker listening for PDF events...")

	// This channel keeps the worker running forever

	go func() {
		for d := range msgs {
			w.processMessage(d)
		}
	}()

	select {}
}

func (w *Worker) processMessage(d amqp.Delivery) {
	var event mq.AnalyzeEvent
	if err := json.Unmarshal(d.Body, &event); err != nil {
		log.Printf("Failed to decode message: %v", err)
		d.Ack(false) // Acknowledge to remove from queue, since it's a bad message
		return
	}

	log.Printf("Processing BookID: %s", event.BookID)

	//update status to "Extracting"
	err := database.DB.Model(&models.Book{}).Where("id = ?", event.BookID).Update("saga_status", "EXTRACTING").Error

	if err != nil {
		log.Printf("DB Update Warning: %v", err)
	}
	log.Println("Step 1: Database status updated to EXTRACTING")

	//download PDF from Minio
	log.Printf("Step 2: Requesting download for %s...", event.PdfS3Key)
	data, err := storage.DownloadFile(context.Background(), event.PdfS3Key)
	if err != nil {
		log.Printf("Failed to download PDF: %v", err)
		database.DB.Model(&models.Book{}).Where("id = ?", event.BookID).Update("saga_status", "ERROR_DOWNLOAD")
		d.Ack(false) //Instant kill if failed to download, since we can't proceed without the PDF
		return
	}
	log.Println("Step 2: Downloaded PDF into memory")

	//extract text from PDF for TOC
	log.Println("Step 3: Starting text extraction...")
	text, err := extractText(data)
	if err != nil {
		log.Printf("Failed to extract text: %v", err)
		database.DB.Model(&models.Book{}).Where("id = ?", event.BookID).Update("saga_status", "ERROR_EXTRACTION")
		d.Ack(false) // Acknowledge to remove from queue, since it's a bad message
		return
	}
	log.Println("Step 3: Text extraction completed")

	log.Printf("Extracted %d characters. Ready for Groq.", len(text))

	log.Println("Step 4: Sending text to Groq AI for analysis...")

	analysisResult, err := llm.AnalyzeBookText(text)
	if err != nil {
		log.Printf("Failed to analyze with Groq: %v", err)
		database.DB.Model(&models.Book{}).Where("id = ?", event.BookID).Update("saga_status", "ERROR_AI")
		d.Ack(false)
		return
	}

	log.Printf("Groq Detected Language: %s", analysisResult.Language)
	log.Printf("Groq Analysis Result: %s", analysisResult.Summary)

	//print the first 100 characters to debug
	if len(text) > 100 {
		log.Printf("Preview: %s...", text[:100])
	}

	/// Step 5: Update DB with AI's language (overwriting whatever the user uploaded!)
	database.DB.Model(&models.Book{}).Where("id = ?", event.BookID).Updates(map[string]interface{}{
		"saga_status": "ANALYZED",
		"summary":     analysisResult.Summary,
		"language":    analysisResult.Language, // The AI is the source of truth now
	})
	log.Println("Step 5: Summary and Detected Language saved.")

	// Pass the baton to Phase 3
	synthEvent := mq.SynthesizeEvent{
		BookID:   event.BookID,
		Text:     analysisResult.Summary,  // We are converting the summary to audio!
		Language: analysisResult.Language, // Update this dynamically if you added Language to AnalyzeEvent
	}

	err = w.Rabbit.PublishSynthesizeEvent(synthEvent)
	if err != nil {
		log.Printf("Failed to trigger Synthesizer: %v", err)
	} else {
		log.Println("Sent to Synthesizer Queue")
	}
	d.Ack(false)
}

func extractText(data []byte) (string, error) {
	reader := bytes.NewReader(data)
	content, err := pdf.NewReader(reader, int64(len(data)))

	if err != nil {
		return "", fmt.Errorf("failed to create pdf reader: %w", err)
	}

	var buf bytes.Buffer
	for i := 1; i <= 5 && i <= content.NumPage(); i++ {
		p := content.Page(i)
		if p.V.IsNull() {
			continue
		}
		t, err := p.GetPlainText(nil)
		if err != nil {
			return "", fmt.Errorf("failed to get text from page %d: %w", i, err)
		}
		buf.WriteString(t)
	}

	return buf.String(), nil
}
