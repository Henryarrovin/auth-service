package data

import (
	"auth-service/models"
	"context"
	"fmt"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u *models.User, roleName string) error {
	var role models.Role
	if err := r.db.WithContext(ctx).Where("name = ?", roleName).First(&role).Error; err != nil {
		return fmt.Errorf("role not found: %w", err)
	}

	u.Roles = []models.Role{role}

	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	err := r.db.WithContext(ctx).
		Preload("Roles").
		Where("email = ?", email).
		First(&u).Error
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*models.User, error) {
	var u models.User
	err := r.db.WithContext(ctx).
		Preload("Roles").
		Where("id = ?", id).
		First(&u).Error
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) AssignRole(ctx context.Context, userID, roleName string) error {
	var user models.User
	if err := r.db.WithContext(ctx).First(&user, "id = ?", userID).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	var role models.Role
	if err := r.db.WithContext(ctx).Where("name = ?", roleName).First(&role).Error; err != nil {
		return fmt.Errorf("role not found: %w", err)
	}

	return r.db.WithContext(ctx).Model(&user).Association("Roles").Append(&role)
}

func (r *UserRepository) GetRoles(ctx context.Context, userID string) ([]string, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Preload("Roles").
		First(&user, "id = ?", userID).Error
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	names := make([]string, len(user.Roles))
	for i, r := range user.Roles {
		names[i] = r.Name
	}
	return names, nil
}
