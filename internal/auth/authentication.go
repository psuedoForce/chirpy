package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func HashPassword(pass string) (string, error) {
	return argon2id.CreateHash(pass, argon2id.DefaultParams)
}

func CheckPasswordHash(password, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn * time.Millisecond)),
	})
	return token.SignedString([]byte(tokenSecret))
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	var res uuid.UUID
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(tokenSecret), nil
	})

	if err != nil {
		return res, err
	}
	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return res, fmt.Errorf("Unauthorized")
	}
	res, e := uuid.Parse(claims.Subject)
	if e != nil {
		return res, fmt.Errorf("Unauthorized")
	}
	return res, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	authString := headers.Get("Authorization")
	fmt.Println("Auth Header")
	fmt.Println(authString)
	str := strings.Fields(authString)
	if len(str) < 2 {
		return "", fmt.Errorf("No Token")
	}
	token := str[1]
	return token, nil
}

func MakeRefreshToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	refresh_token := hex.EncodeToString(b)
	return refresh_token
}

func GetAPIKey(headers http.Header) (string, error) {
	authString := headers.Get("Authorization")
	fields := strings.Fields(authString)
	if len(fields) < 2 {
		return "", fmt.Errorf("No API Key found")
	}
	return fields[1], nil
}
