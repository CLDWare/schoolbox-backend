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
	ActiveQuestionID uint
	ActiveQuestion   Question `gorm:"foreignKey:ActiveQuestionID;references:ID"`
	// Device lease
	LeaseStart   time.Time
	ActiveUserID *uint
	ActiveUser   *User `gorm:"foreignKey:ActiveUserID;references:ID"`
}

type User struct {
	gorm.Model
	Email           string `gorm:"unique"`
	Name            string
	Role            uint
	DefaultQuestion string `gorm:"default:'Wat vond je van de les?'"`
}

type Question struct {
	gorm.Model
	Question string `gorm:"unique;default:'Wat vond je van de les?'"`
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
	FirstAnwserTime time.Time
	LastAnwserTime  time.Time
	A1_count        uint16
	A2_count        uint16
	A3_count        uint16
	A4_count        uint16
	A5_count        uint16
}
