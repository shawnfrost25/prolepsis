package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func CreateToken() (string, error) {
	raw := make([]byte, 32)
	_, err := rand.Read(raw)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(raw), nil
}

func HashToken(token string) string {
	clean := strings.TrimSpace(token)
	hash := sha256.Sum256([]byte(clean))
	return hex.EncodeToString(hash[:])
}
