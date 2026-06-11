package data

import (
	"context"
	"fmt"

	"github.com/Henryarrovin/auth-service/models"

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

func (r *UserRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	result := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("password_hash", passwordHash)

	if result.Error != nil {
		return result.Error
	}
	return nil
}

// CreateOAuthUser creates a user without password (OAuth only)
func (r *UserRepository) CreateOAuthUser(ctx context.Context, u *models.User) error {
	var role models.Role
	if err := r.db.WithContext(ctx).Where("name = ?", models.RoleUser).First(&role).Error; err != nil {
		return fmt.Errorf("role not found: %w", err)
	}

	u.Roles = []models.Role{role}

	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		return fmt.Errorf("create oauth user: %w", err)
	}
	return nil
}

// UpdateOAuthInfo links an OAuth provider to an existing local account
func (r *UserRepository) UpdateOAuthInfo(ctx context.Context, userID, provider, providerID, avatarURL string) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"provider":    provider,
			"provider_id": providerID,
			"avatar_url":  avatarURL,
		}).Error
}

func (r *UserRepository) Update2FASecret(ctx context.Context, userID, secret string) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("two_fa_secret", secret).Error
}

func (r *UserRepository) Enable2FA(ctx context.Context, userID string, backupCodes string) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"two_fa_enabled":      true,
			"two_fa_backup_codes": backupCodes,
		}).Error
}

func (r *UserRepository) Disable2FA(ctx context.Context, userID string) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"two_fa_enabled":      false,
			"two_fa_secret":       "",
			"two_fa_backup_codes": "",
		}).Error
}
