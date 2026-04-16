package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// user owns the authentication state
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Email        string    `gorm:"uniqueIndex;not null"`
	PasswordHash string    `gorm:"not null"`
	Role         string    `gorm:"default:'USER'"`
	CreateAt     time.Time
	UpdateAt     time.Time
	DeleteAt     gorm.DeletedAt `gorm:"index"`
}

// book represents top level entity
// doesn't have a hasMany relationships to chapters here!
type Book struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Title    string    `gorm:"not null"`
	Author   string
	Language string `gorm:"not null"`
	PdfS3Key string `gorm:"not null"`
	//saga Tracking
	// States: UPLOAD_PENDING -> ANALYZING -> SYNTHESIZING -> PUBLISHED -> FAILED
	SagaStatus string `gorm:"not null; default: 'UPLOAD_PENDING'"`
	CreateAt   time.Time
	UpdateAt   time.Time
	DeleteAt   gorm.DeletedAt `gorm:"index"`
}
