package main

import (
	"log"
	"os"

	"adeff/internal/analyzer"
	"adeff/internal/database"
	"adeff/pkg/mq"
	"adeff/pkg/storage"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("No .env file found: %v", err)
	}

	// initialize dependencies
	database.InitGatewayDB()
	storage.InitMinio()

	//connect to CloudAMQP
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		log.Fatal("Error: RABBITMQ_URL is not set.")
	}

	rabbitClient, err := mq.ConnectRabbitMQ(rabbitURL)
	if err != nil {
		log.Fatal("Failed connecting to RabbitMQ: %v", err)
	}

	defer rabbitClient.Conn.Close()
	defer rabbitClient.Channel.Close()

	//start the worker
	worker := &analyzer.Worker{
		Rabbit: rabbitClient,
	}

	if err := worker.Start(); err != nil {
		log.Fatal("Worker error: %v", err)
	}

}
