package jwt

import (
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type (
	BaseInfo struct {
		UserID   int64  `json:"userId"`
		Username string `json:"username"`
	}

	JwtClaims struct {
		BaseInfo
		jwtv5.RegisteredClaims
	}
)
