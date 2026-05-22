package main

import (
	"log"
	"net/http"
	"os"

	_ "adeff/docs"
	"adeff/internal/database"
	"adeff/internal/models"
	"adeff/pkg/mq"
	"adeff/pkg/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Handler holds dependencies for API handlers
type Handler struct {
	Rabbit *mq.RabbitClient
}

// @title Adeff API Gateway
// version 1.0
// @description The central API Gateway for the Adeff distributed audiobook platform.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

//@license.name MIT
//@license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /
func main() {
	// loading env variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found. Falling back to system environment variables.")
	}

	if os.Getenv("NEON_GATEWAY_DSN") == "" {
		log.Fatal("Error: NEON_GATEWAY_DSN environment variable is not set.")
	}

	// Initialize database connection
	database.InitGatewayDB()
	// Initialize MinIO client
	storage.InitMinio()

	// Initialize CloudAMQP
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		log.Fatal("Error: RABBITMQ_URL environment variable is not set.")
	}

	rabbitClient, err := mq.ConnectRabbitMQ(rabbitURL)
	if err != nil {
		log.Fatal("Error connecting to RabbitMQ:", err)
	}
	defer rabbitClient.Conn.Close()
	defer rabbitClient.Channel.Close()

	//setup handler with dependencies
	h := &Handler{
		Rabbit: rabbitClient,
	}

	//initailizing gin routers
	router := gin.Default()

	//global middlewares
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	//swagger router
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// health check endpoint
	router.GET("/health", HealthCheck)

	v1 := router.Group("/api/v1")
	{
		adminGroup := v1.Group("/admin")
		{
			adminGroup.POST("/upload", h.handleAdminUpload)
		}
	}

	port := ":8080"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = ":" + envPort
	}
	log.Printf("Starting Adeff API Gateway on port %s", port)

	if err := router.Run(port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// HealthCheck godoc
// @Summary Check system health
// @Description Returns the operational status of the API Gateway
// @Tags System
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health [get]
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "online",
		"service": "adeff-api-gateway",
		"message": "System is ready for audiobook processing",
	})
}

// handleAdminUpload godoc
// @Summary Upload a PDF to trigger audio generation
// @Description Initiates the Saga pattern. Uploads PDF to MinIO, creates DB record, and triggers the Analyzer worker.
// @Tags Admin
// @Accept multipart/form-data
// @Produce json
// @Param title formData string true "Title of the book"
// @Param file formData file true "The PDF file"
// @Success 202 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/admin/upload [post]
func (h *Handler) handleAdminUpload(c *gin.Context) {
	title := c.PostForm("title")
	language := c.PostForm("language")

	fileHeader, err := c.FormFile("file")
	if err != nil {
		log.Printf("[Gateway] Missing file in upload payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "PDF File is required"})
		return
	}

	fileStream, err := fileHeader.Open()
	if err != nil {
		log.Printf("[Gateway] Failed to open uploaded file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process uploaded file"})
		return
	}
	defer fileStream.Close()

	// upload to s3 minio
	MinioS3Key := "temp-uploads/" + uuid.New().String() + ".pdf"

	err = storage.UploadStream(c.Request.Context(), MinioS3Key, fileStream, fileHeader.Size, "application/pdf")
	if err != nil {
		log.Printf("[Gateway] Failed to upload file to MinIO: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload file to CDN"})
		return
	}

	// creating a book in gatewaydb and saving to neondb
	book := models.Book{
		Title:      title,
		Language:   language,
		PdfS3Key:   MinioS3Key,
		SagaStatus: "ANALYZING",
	}

	result := database.DB.Create(&book)
	if result.Error != nil {
		log.Printf("[Gateway] Database error: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create book record"})
		return
	}

	// 5. Publish Event to CloudAMQP
	event := mq.AnalyzeEvent{
		BookID:   book.ID.String(),
		PdfS3Key: MinioS3Key,
	}

	err = h.Rabbit.PublishAnalyzeEvent(event)
	if err != nil {
		log.Printf("[Gateway] Failed to publish ClouudAMQP event: %v", err)
	} else {
		log.Printf("[Gateway] Published AnalyzeEvent to RabbitMQ for BookID: %s", book.ID.String())
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Upload successful. Saga initiated.",
		"book_id": book.ID,
		"status":  book.SagaStatus,
	})
}

func publishAnalyzeEvent(bookID string, s3Key string) {
	log.Printf("Published to RabbitMQ -> BookID: %s, Key: %s", bookID, s3Key)
}
