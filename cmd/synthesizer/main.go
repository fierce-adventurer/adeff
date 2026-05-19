package main

import (
	"log"
	"os"

	"adeff/internal/database"
	"adeff/internal/synthesizer"
	"adeff/pkg/mq"
	"adeff/pkg/storage"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found.")
	}

	database.InitGatewayDB()
	storage.InitMinio()

	rabbitURL := os.Getenv("RABBITMQ_URL")
	rabbitClient, err := mq.ConnectRabbitMQ(rabbitURL)
	if err != nil {
		log.Fatalf("Failed to connect to CloudAMQP: %v", err)
	}
	defer rabbitClient.Conn.Close()
	defer rabbitClient.Channel.Close()

	worker := &synthesizer.Worker{
		Rabbit: rabbitClient,
	}

	if err := worker.Start(); err != nil {
		log.Fatalf("Synthesizer Worker crashed: %v", err)
	}
}
