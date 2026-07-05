package oauth_service

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Henryarrovin/auth-service/config"
	"github.com/Henryarrovin/auth-service/data"
	"github.com/Henryarrovin/auth-service/middleware"
	"github.com/Henryarrovin/auth-service/models"
	jwt_service "github.com/Henryarrovin/auth-service/services/jwt_service"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

type OAuthService struct {
	users      *data.UserRepository
	tokenStore *data.TokenStore
	jwt        *jwt_service.JWTService
	logger     *zap.Logger
	configs    map[string]*oauth2.Config
}

func NewOAuthService(
	cfg config.OAuthConfig,
	users *data.UserRepository,
	tokenStore *data.TokenStore,
	jwt *jwt_service.JWTService,
	logger *zap.Logger,
) *OAuthService {
	configs := map[string]*oauth2.Config{
		"google": {
			ClientID:     cfg.Google.ClientID,
			ClientSecret: cfg.Google.ClientSecret,
			RedirectURL:  cfg.Google.RedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
		"github": {
			ClientID:     cfg.GitHub.ClientID,
			ClientSecret: cfg.GitHub.ClientSecret,
			RedirectURL:  cfg.GitHub.RedirectURL,
			Scopes:       []string{"user:email"},
			Endpoint:     github.Endpoint,
		},
	}

	return &OAuthService{
		users:      users,
		tokenStore: tokenStore,
		jwt:        jwt,
		logger:     logger,
		configs:    configs,
	}
}

func (s *OAuthService) GetRedirectURL(ctx context.Context, provider, appRedirectURI string) (string, error) {
	log := middleware.FromContext(ctx, s.logger)

	cfg, ok := s.configs[provider]
	if !ok {
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}

	// random token to prevent CSRF
	state, err := randomHex(16)
	if err != nil {
		return "", err
	}

	if err := s.tokenStore.SaveOAuthState(ctx, state, provider, appRedirectURI); err != nil {
		return "", fmt.Errorf("saving oauth state: %w", err)
	}

	url := cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
	log.Info("oauth redirect", zap.String("provider", provider))
	return url, nil
}

type OAuthResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	IsNewUser    bool
}

func (s *OAuthService) HandleCallback(ctx context.Context, provider, code, state string) (*OAuthResult, error) {
	log := middleware.FromContext(ctx, s.logger)

	// Validate state
	savedProvider, _, err := s.tokenStore.GetOAuthState(ctx, state)
	if err != nil || savedProvider != provider {
		return nil, fmt.Errorf("invalid oauth state")
	}
	_ = s.tokenStore.DeleteOAuthState(ctx, state)

	cfg, ok := s.configs[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	// Exchange code for token
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		log.Error("oauth code exchange failed", zap.Error(err))
		return nil, fmt.Errorf("exchanging oauth code: %w", err)
	}

	// Get user info from provider
	userInfo, err := s.getUserInfo(ctx, provider, token)
	if err != nil {
		return nil, err
	}

	log.Info("oauth user info retrieved",
		zap.String("provider", provider),
		zap.String("email", userInfo.Email),
	)

	// Find or create user
	user, isNew, err := s.findOrCreateUser(ctx, provider, userInfo)
	if err != nil {
		return nil, err
	}

	// Issue JWT pair
	pair, err := s.issuePair(ctx, user)
	if err != nil {
		return nil, err
	}

	log.Info("oauth login successful",
		zap.String("provider", provider),
		zap.String("user_id", user.ID),
		zap.Bool("is_new_user", isNew),
	)

	return &OAuthResult{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    pair.ExpiresIn,
		IsNewUser:    isNew,
	}, nil
}

type providerUserInfo struct {
	ID        string
	Email     string
	Name      string
	AvatarURL string
}

func (s *OAuthService) getUserInfo(ctx context.Context, provider string, token *oauth2.Token) (*providerUserInfo, error) {
	switch provider {
	case "google":
		return s.getGoogleUserInfo(ctx, token)
	case "github":
		return s.getGithubUserInfo(ctx, token)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func (s *OAuthService) getGoogleUserInfo(ctx context.Context, token *oauth2.Token) (*providerUserInfo, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("getting google user info: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var info struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parsing google user info: %w", err)
	}

	return &providerUserInfo{
		ID:        info.ID,
		Email:     info.Email,
		Name:      info.Name,
		AvatarURL: info.Picture,
	}, nil
}

func (s *OAuthService) getGithubUserInfo(ctx context.Context, token *oauth2.Token) (*providerUserInfo, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

	// Get user profile
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("getting github user info: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var info struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parsing github user info: %w", err)
	}

	if info.Email == "" {
		info.Email, _ = s.getGithubPrimaryEmail(client)
	}

	name := info.Name
	if name == "" {
		name = info.Login
	}

	return &providerUserInfo{
		ID:        fmt.Sprintf("%d", info.ID),
		Email:     info.Email,
		Name:      name,
		AvatarURL: info.AvatarURL,
	}, nil
}

func (s *OAuthService) getGithubPrimaryEmail(client *http.Client) (string, error) {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var emails []struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
	}
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", err
	}

	for _, e := range emails {
		if e.Primary {
			return e.Email, nil
		}
	}
	return "", fmt.Errorf("no primary email found")
}

func (s *OAuthService) findOrCreateUser(ctx context.Context, provider string, info *providerUserInfo) (*models.User, bool, error) {
	log := middleware.FromContext(ctx, s.logger)

	// Try find by email
	user, err := s.users.FindByEmail(ctx, info.Email)
	if err == nil {
		// User exists — update provider info if needed
		if user.Provider == "local" {
			log.Info("linking oauth to existing local account",
				zap.String("email", info.Email),
				zap.String("provider", provider),
			)
			_ = s.users.UpdateOAuthInfo(ctx, user.ID, provider, info.ID, info.AvatarURL)
		}
		return user, false, nil
	}

	// Create new user
	log.Info("creating new oauth user",
		zap.String("email", info.Email),
		zap.String("provider", provider),
	)

	newUser := &models.User{
		Email:      info.Email,
		Name:       info.Name,
		Provider:   provider,
		ProviderID: info.ID,
		AvatarURL:  info.AvatarURL,
	}

	if err := s.users.CreateOAuthUser(ctx, newUser); err != nil {
		return nil, false, fmt.Errorf("creating oauth user: %w", err)
	}

	return newUser, true, nil
}

func (s *OAuthService) issuePair(ctx context.Context, user *models.User) (*TokenPair, error) {
	access, err := s.jwt.IssueAccessToken(user)
	if err != nil {
		return nil, err
	}

	tokenHash, err := randomHex(32)
	if err != nil {
		return nil, err
	}

	refresh, err := s.jwt.IssueRefreshToken(user.ID, tokenHash)
	if err != nil {
		return nil, err
	}

	if err := s.tokenStore.Save(ctx, user.ID, tokenHash); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(s.jwt.AccessTTL().Seconds()),
	}, nil
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := crypto_rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *OAuthService) PeekAppRedirect(ctx context.Context, state string) (string, error) {
	_, appRedirectURI, err := s.tokenStore.GetOAuthState(ctx, state)
	if err != nil {
		return "", err
	}
	return appRedirectURI, nil
}
