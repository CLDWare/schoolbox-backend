package main

import (
	"context"
	"time"

	models "github.com/CLDWare/schoolbox-backend/pkg/db"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect to database: " + err.Error())
	}

	ctx := context.Background()

	db.AutoMigrate(&models.Device{}, &models.User{}, &models.Question{}, &models.Session{})

	// DUMMY DATA
	device1 := models.Device{
		Token: "a1b2c3",
	}
	err = gorm.G[models.Device](db).Create(ctx, &device1)

	user1 := models.User{
		Email: "t.vandervelden@chrlyceumdelft.nl ",
		Name:  "Tom van der Velden",
		Role:  1,
	}
	err = gorm.G[models.User](db).Create(ctx, &user1)

	question1 := models.Question{Question: "Wat vond je van de les?"}
	err = gorm.G[models.Question](db).Create(ctx, &question1)

	session1 := models.Session{
		UserID:          user1.ID,
		QuestionID:      question1.ID,
		DeviceID:        device1.ID,
		Date:            time.Now().Add(-15 * time.Minute), // Session was started 15 minutes ago,
		FirstAnwserTime: time.Now().Add(-10 * time.Minute), // first question answered 10 minutes ago
		LastAnwserTime:  time.Now().Add(-5 * time.Minute),  // last question answered 5 minutes ago
		A1_count:        0,
		A2_count:        1,
		A3_count:        7,
		A4_count:        10,
		A5_count:        5,
	}
	gorm.G[models.Session](db).Create(ctx, &session1)

	user, err := gorm.G[models.User](db).Where("id = ?", 1).First(ctx)
	println(user.ID, user.Email, user.Name, user.DefaultQuestion)
}
