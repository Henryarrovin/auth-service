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

type UserProvider struct {
	ID         string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID     string `gorm:"type:uuid;not null"`
	Provider   string `gorm:"not null"`
	ProviderID string `gorm:"not null"`
	AvatarURL  string
	CreatedAt  time.Time
}

type User struct {
	ID               string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Email            string `gorm:"uniqueIndex;not null"`
	Name             string `gorm:"not null"`
	PasswordHash     string // empty for OAuth users
	Provider         string `gorm:"default:'local'"` // "local" | "google" | "github"
	ProviderID       string // OAuth provider's user ID
	AvatarURL        string // profile picture from OAuth
	TwoFASecret      string // TOTP secret
	TwoFAEnabled     bool   `gorm:"default:false"`
	TwoFABackupCodes string // JSON array of hashed backup codes
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Roles            []Role         `gorm:"many2many:user_roles;"`
	Providers        []UserProvider `gorm:"foreignKey:UserID"`
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

func (u *User) RoleNames() []string {
	names := make([]string, len(u.Roles))
	for i, r := range u.Roles {
		names[i] = r.Name
	}
	return names
}

func (u *User) IsOAuthUser() bool {
	return u.Provider != "local" && u.Provider != ""
}

func (u *User) HasProvider(provider string) bool {
	for _, p := range u.Providers {
		if p.Provider == provider {
			return true
		}
	}
	return false
}

func (u *User) HasPassword() bool {
	return u.PasswordHash != ""
}
