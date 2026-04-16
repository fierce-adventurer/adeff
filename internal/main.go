package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	//initailizing gin routers
	router := gin.Default()

	//global middlewares
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "online",
			"service": "adeff-api-gateway",
			"message": "System is ready for audiobook processing",
		})
	})

	port := ":8080"
	log.Printf("Starting Adeff API Gateway on port %s", port)

	if err := router.Run(port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
