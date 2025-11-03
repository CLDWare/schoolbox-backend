package db

import (
	"context"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect to database: " + err.Error())
	}

	ctx := context.Background()

	db.AutoMigrate(&Device{}, &User{}, &Question{}, &Session{})

	// DUMMY DATA
	device1 := Device{
		Token: "a1b2c3",
	}
	err = gorm.G[Device](db).Create(ctx, &device1)

	user1 := User{
		Email: "t.vandervelden@chrlyceumdelft.nl ",
		Name:  "Tom van der Velden",
		Role:  1,
	}
	err = gorm.G[User](db).Create(ctx, &user1)

	question1 := Question{Question: "Wat vond je van de les?"}
	err = gorm.G[Question](db).Create(ctx, &question1)

	session1 := Session{
		UserID:          user1.ID,
		QuestionID:      question1.ID,
		DeviceID:        device1.ID,
		Date:            time.Now().Add(-15 * time.Minute), // Session was started 15 minutes ago,
		FirstAnwserTime: time.Now().Add(-10 * time.Minute), // first question answered 10 minutes ago
		LastAnwserTime:  time.Now().Add(-5 * time.Minute),  // last question answered 5 minutes ago
		a1_count:        0,
		a2_count:        1,
		a3_count:        7,
		a4_count:        10,
		a5_count:        5,
	}
	gorm.G[Session](db).Create(ctx, &session1)

	user, err := gorm.G[User](db).Where("id = ?", 1).First(ctx)
	println(user.ID, user.Email, user.Name, user.DefaultQuestion)
}
