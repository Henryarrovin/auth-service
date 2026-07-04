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

type LoginResult struct {
	// Set when 2FA not required
	Pair *TokenPair

	// Set when 2FA required
	Requires2FA bool
	TempToken   string
}

type RefreshResult struct {
	// Set when 2FA not required
	Pair *TokenPair

	// Set when 2FA required
	Requires2FA bool
	TempToken   string
}

type Setup2FAResult struct {
	Secret  string
	QRURL   string
	QRImage string
}

// SyncKeyMaterial is what a device needs to re-derive the key-encryption key
// locally and unwrap the data key. Configured is false until any device has
// run SetupSyncKey for this account.
type SyncKeyMaterial struct {
	Configured      bool
	Salt            string
	KDFParams       string
	WrappedDEK      string
	WrappedDEKNonce string
}
