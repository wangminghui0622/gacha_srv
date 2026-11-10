// Package jwtutil JWT 签发与校验（HS256，密钥来自配置占位符，生产替换）
package jwtutil

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// Claims 自定义载荷：网关鉴权中间件从 ctx 拿到的用户身份
type Claims struct {
	UserId   int64  `json:"userId"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	jwt.RegisteredClaims
}

// NewToken 签发 token；expire 单位为秒
func NewToken(secret string, expire int64, userId int64, username, nickname string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserId:   userId,
		Username: username,
		Nickname: nickname,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "gacha-user",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(expire) * time.Second)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParseToken 校验并解析 token；失败返回错误（调用方转 401）
func ParseToken(secret, token string) (*Claims, error) {
	claims := &Claims{}
	t, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !t.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
