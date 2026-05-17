package email_service

import "github.com/google/wire"

var ProviderSet = wire.NewSet(NewEmailService)
