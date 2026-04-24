package data

import (
	"auth-service/config"
	"auth-service/models"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewDB(cfg *config.Config, logger *zap.Logger) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN()), &gorm.Config{
		Logger: NewGormLogger(logger),
	})
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	if err := db.AutoMigrate(&models.User{}, &models.Role{}, &models.UserRole{}); err != nil {
		return nil, fmt.Errorf("auto migrating: %w", err)
	}

	// Seed default roles
	roles := []models.Role{
		{Name: "admin"},
		{Name: "moderator"},
		{Name: "user"},
	}
	for _, r := range roles {
		db.Where(models.Role{Name: r.Name}).FirstOrCreate(&r)
	}

	return db, nil
}
