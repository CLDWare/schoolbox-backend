package db

import (
	"time"

	"gorm.io/gorm"
)

type Device struct {
	gorm.Model
	ActiveUserID     *uint
	ActiveUser       *User `gorm:"foreignKey:ActiveUserID;references:ID"`
	Token            string
	Room             *string `gorm:"unique"`
	ActiveQuestionID uint
	ActiveQuestion   Question `gorm:"foreignKey:ActiveQuestionID;references:ID"`
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
	UserID          uint
	User            User `gorm:"foreignKey:UserID;references:ID"`
	QuestionID      uint
	Question        Question `gorm:"foreignKey:QuestionID;references:ID"`
	DeviceID        uint
	Device          Device `gorm:"foreignKey:DeviceID;references:ID"`
	Date            time.Time
	FirstAnwserTime time.Time
	LastAnwserTime  time.Time
	a1_count        uint16
	a2_count        uint16
	a3_count        uint16
	a4_count        uint16
	a5_count        uint16
}
