package janitor

import (
	"context"
	"fmt"
	"time"

	"github.com/CLDWare/schoolbox-backend/config"
	models "github.com/CLDWare/schoolbox-backend/pkg/db"
	"github.com/CLDWare/schoolbox-backend/pkg/logger"
	"gorm.io/gorm"
)

type Janitor struct {
	cfg              *config.Config
	database         *gorm.DB
	announceNoAction bool
	cancel           context.CancelFunc
}

func NewJanitor(cfg *config.Config, db *gorm.DB, announceNoAction bool) *Janitor {
	return &Janitor{
		cfg:              cfg,
		database:         db,
		announceNoAction: announceNoAction,
	}
}

func (jan *Janitor) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	jan.cancel = cancel

	go func() {
		shortTicker := time.NewTicker(jan.cfg.Janitor.ShortCleanInterval)
		defer shortTicker.Stop()
		fullTicker := time.NewTicker(jan.cfg.Janitor.FullCleanInterval)
		defer fullTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-shortTicker.C:
				jan.RunShort()
			case <-fullTicker.C:
				jan.RunFull()
			}
		}
	}()
}

func (jan *Janitor) Stop() {
	if jan.cancel != nil {
		jan.cancel()
		jan.cancel = nil
	}
}

func (jan *Janitor) RunShort() {
	logger.Info("Janitor: Running short cleaning sequence.")
	jan.CleanUpExpiredAuthSession()

}

func (jan *Janitor) RunFull() {
	logger.Info("Janitor: Running full cleaning sequence.")
	jan.RunShort()

	jan.DeepCleanDatabase(nil)
}

// DeepCleanDatabase forces gorm to delete all "deleted" entries
func (jan *Janitor) DeepCleanDatabase(deepcleanModels *[]any) {
	if deepcleanModels == nil {
		deepcleanModels = &[]any{
			models.Device{},
			models.User{},
			models.AuthSession{},
			models.Question{},
			models.Session{},
		}
	}
	for _, deepcleanModel := range *deepcleanModels {
		result := jan.database.Unscoped().Where("deleted_at IS NOT NULL").Delete(deepcleanModel)
		if result.Error != nil {
			logger.Err(fmt.Sprintf("Janitor: Error while deepcleaning model %t: %s", deepcleanModel, result.Error.Error()))
		} else {
			if jan.announceNoAction || result.RowsAffected != 0 {
				logger.Info(fmt.Sprintf("Janitor: Deleted %d rows from model %T", result.RowsAffected, deepcleanModel))
			}
		}
	}
}

// CleanUpExpiredAuthSession cleans up auth sessions that have expired
func (jan *Janitor) CleanUpExpiredAuthSession() {
	ctx := context.Background()

	sessionsDeleted, err := gorm.G[models.AuthSession](jan.database).Where("expires_at < ?", time.Now()).Delete(ctx)
	if err != nil {
		return
	}
	logger.Info(fmt.Sprintf("Janitor: cleaned %d expired auth sessions", sessionsDeleted))
}
