package db

import (
	"time"

	"gorm.io/gorm"
)

type Device struct {
	gorm.Model
	RegistrationDate time.Time
	LatestLogin      *time.Time
	LastSeen         *time.Time
	Token            string
	Room             *string `gorm:"unique"`
	// Device lease
	LeaseStart      time.Time
	ActiveSessionID *uint
	ActiveSession   *Session `gorm:"foreignKey:ActiveSessionID;references:ID"`
}

type User struct {
	gorm.Model
	Email           string `gorm:"unique"`
	GoogleSubject   string `gorm:"unique"` // "sub" field from the OAuth response, should be unique
	ProfilePicture  string // url to the users pfp (straight from google)
	Name            string
	DisplayName     string
	Role            uint   // 0: default user, 1: admin
	DefaultQuestion string `gorm:"default:'Wat vond je van de les?'"`
}

type AuthSession struct {
	gorm.Model
	SessionToken string
	ExpiresAt    time.Time
	UserID       uint
	User         User `gorm:"foreignKey:UserID;references:ID"`
}

type Question struct {
	gorm.Model
	Question string `gorm:"unique;default:'Wat vond je van de les?'"`
	UserID   uint   // Who owns this question
	User     User   `gorm:"foreignKey:UserID;references:ID"`
}

type Session struct {
	gorm.Model
	UserID          uint
	User            User `gorm:"foreignKey:UserID;references:ID"`
	QuestionID      uint
	Question        Question `gorm:"foreignKey:QuestionID;references:ID"`
	DeviceID        uint
	Device          Device `gorm:"foreignKey:DeviceID;references:ID"`
	Date            time.Time
	FirstAnwserTime *time.Time
	LastAnwserTime  *time.Time
	StoppedAt       *time.Time
	A1_count        uint16
	A2_count        uint16
	A3_count        uint16
	A4_count        uint16
	A5_count        uint16
}
