package auth

import (
	"crypto/rand"
	"encoding/hex"
)

func CreateToken() (string, error) {
	raw := make([]byte, 32)
	_, err := rand.Read(raw)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(raw), nil
}
