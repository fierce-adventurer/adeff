package database

import (
	"log"
	"os"

	"adeff/interval/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitGatewayDB() {
	dsn := os.Getenv("NEON_GATEWAY_DSN")
	if dsn == "" {
		log.Fatal("NEON_GATEWAY_DSN environment variable is not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to the database %v", err)
	}

	err = db.AutoMigrate(&models.User{}, &models.Book{})
	if err != nil {
		log.Fatal("Failed to migrate the database %v", err)
	}
	DB = db
	log.Println("Successfully connected to NeonDB (Gateway Database)")
}
