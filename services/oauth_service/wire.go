package oauth_service

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewOAuthService,
)
