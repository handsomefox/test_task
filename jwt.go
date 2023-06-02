package main

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

const (
	JWTAccessTokenExpirationTime = time.Hour * 12
	JWTSecretEnv                 = "JWT_SECRET_KEY"
	JWTIssuer                    = "test_task_app"
)

type Token struct {
	Access string `json:"access_token"`
}

func NewJWTAccessToken(user User) (*Token, error) {
	now := time.Now()
	claims := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.RegisteredClaims{
		Issuer:    JWTIssuer,
		ExpiresAt: jwt.NewNumericDate(now.Add(JWTAccessTokenExpirationTime)),
		IssuedAt:  &jwt.NumericDate{Time: now},
		Audience:  jwt.ClaimStrings{user.Username},
		ID:        fmt.Sprint(user.ID),
	})

	secret := []byte(os.Getenv(JWTSecretEnv))

	token, err := claims.SignedString(secret)
	if err != nil {
		return nil, err
	}

	return &Token{Access: token}, nil
}

func VerifyJWTToken(token string) (*jwt.RegisteredClaims, bool) {
	var (
		claims = &jwt.RegisteredClaims{}
		secret = []byte(os.Getenv(JWTSecretEnv))
	)

	tkn, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (any, error) { return secret, nil })
	if err != nil {
		return nil, false
	}

	return claims, tkn.Valid
}
