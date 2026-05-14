package analyzer

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
		d.Nack(false, false) 
		return
	}

	log.Printf("Processing BookID: %s", event.BookID)

	//update status to "Extracting"
	database.DB.Model(&models.Book{}).Where("id = ?", event.BookID).Update("saga_status", "EXTRACTING")

	//download PDF from Minio
	data, err := storage.DownloadFile(context.Background(), event.PdfS3Key)
	if err != nil {
		log.Printf("Failed to download PDF: %v", err)
		d.Nack(false, true) // Requeue if it's a transient network error
		return
	}

	//extract text from PDF for TOC

	text, err := extractText(data)
	if err != nil {
		log.Printf("Failed to extract text: %v", err)
		database.DB.Model(&models.Book{}).Where("id = ?", event.BookID).Update("saga_status", "ERROR_EXTRACTION")
		d.Ack(false)
		return
	}

	log.Printf("Extracted %d characters. Ready for Groq.", len(text))

	//Send text to Groq for TOC extraction

	database.DB.Model(&models.Book{}).Where("id = ?", event.BookID).Update("saga_status", "ANALYZING_AI")
	d.Ack(false)
}

func extractText(data []byte) (string, error) {
	reader := bytes.NewReader(data)
	content, err := pdf.NewReader(reader, int64(len(data)))

	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	for i := 1 ; i <= 5 && i <= content.NumPage(); i++ {
		p := content.Page(i)
		if p.V.IsNull() {
			continue
		}
		t , _ := p.GetPlainText(nil)
		buf.WriteString(t)
	}

	return buf.String(), nil
}
