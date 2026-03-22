package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func generateSecret(bytes int) string {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func main() {
	fmt.Println("access_secret:   ", generateSecret(32))
	fmt.Println("refresh_secret:  ", generateSecret(32))
	fmt.Println("canonical_secret:", generateSecret(32))
}
