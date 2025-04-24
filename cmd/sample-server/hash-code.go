package main

import (
	"encoding/base64"

	"golang.org/x/crypto/argon2"
)

const (
	SaltyPhrase = "choosing random salts is hard"
)

// Converts request password to a hash byte array using `Strength` parameter
func hash(request *HashRequest) []byte {
	hash := argon2.IDKey([]byte(request.Password), []byte(SaltyPhrase), uint32(request.Strength), 64*1024, 4, 32)
	return hash
}

// Converts request password to a Base64 string of the password hash
func hashStr(request *HashRequest) string {
	return base64.StdEncoding.EncodeToString(hash(request))
}
