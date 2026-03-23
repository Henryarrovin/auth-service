package models

import "time"

// Roles
const (
	RoleAdmin     = "admin"
	RoleModerator = "moderator"
	RoleUser      = "user"
)

type Role struct {
	ID        string    `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
}

type User struct {
	ID           string    `db:"id"`
	Email        string    `db:"email"`
	Name         string    `db:"name"`
	PasswordHash string    `db:"password_hash"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
	Roles        []string  `db:"-"`
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
