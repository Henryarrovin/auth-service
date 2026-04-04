package auth_service

type RegisterInput struct {
	Email    string
	Password string
	Name     string
	Role     string // optional; defaults to "user"
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64 // seconds
}

type ValidateResult struct {
	UserID string
	Email  string
	Roles  []string
}
