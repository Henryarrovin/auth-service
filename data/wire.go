package data

import (
	"github.com/google/wire"
	"gorm.io/gorm"
)

var ProviderSet = wire.NewSet(
	NewDB,
	NewTokenStore,
	NewUserRepository,
	NewCleanup,
)

func NewCleanup(db *gorm.DB, tokenStore *TokenStore) func() {
	return func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
		tokenStore.rdb.Close()
	}
}
