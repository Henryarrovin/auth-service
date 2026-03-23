package models

import "time"

const (
	RoleAdmin     = "admin"
	RoleModerator = "moderator"
	RoleUser      = "user"
)

type Role struct {
	ID        string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name      string `gorm:"uniqueIndex;not null"`
	CreatedAt time.Time
}

type User struct {
	ID           string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Email        string `gorm:"uniqueIndex;not null"`
	Name         string `gorm:"not null"`
	PasswordHash string `gorm:"not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Roles        []Role `gorm:"many2many:user_roles;"`
}

// UserRole is the join table
type UserRole struct {
	UserID string `gorm:"type:uuid;primaryKey"`
	RoleID string `gorm:"type:uuid;primaryKey"`
}

// AccessClaims are embedded in the short-lived access token.
type AccessClaims struct {
	UserID string   `json:"uid"`
	Email  string   `json:"email"`
	Roles  []string `json:"roles"`
}

// RefreshClaims are embedded in the long-lived refresh token.
type RefreshClaims struct {
	UserID    string `json:"uid"`
	TokenHash string `json:"jti"`
}
