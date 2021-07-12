/*
Copyright 2021 KubeCube Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package jwt

import (
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/env"
	"time"

	"k8s.io/api/authentication/v1beta1"

	"github.com/dgrijalva/jwt-go"
	"github.com/kubecube-io/kubecube/pkg/authenticator"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/errcode"
)

type Claims struct {
	UserInfo v1beta1.UserInfo
	jwt.StandardClaims
}

const BearerTokenPrefix = "Bearer"

var Config = authenticator.JwtConfig{}

func init() {
	Config = authenticator.JwtConfig{
		JwtSecret: env.JwtSecret(),
	}
}

// GenerateToken todo: need rewrite
func GenerateToken(name string, expireDuration int64) (string, *errcode.ErrorInfo) {
	var tokenExpireDuration int64 = constants.DefaultTokenExpireDuration
	if Config.TokenExpireDuration > 0 {
		tokenExpireDuration = Config.TokenExpireDuration
	}
	if expireDuration > 0 {
		tokenExpireDuration = expireDuration
	}

	claims := Claims{
		UserInfo: v1beta1.UserInfo{
			Username: name,
			Groups:   []string{constants.KubeCube},
		},
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Unix() + tokenExpireDuration,
			Issuer:    Config.JwtIssuer,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, signErr := token.SignedString([]byte(Config.JwtSecret))
	if signErr != nil {
		clog.Error("sign token with jwt secret error: %s", signErr)
		return "", errcode.InternalServerError
	}
	return signedToken, nil
}

func ParseToken(token string) (Claims, *errcode.ErrorInfo) {
	claims := &Claims{}

	// Empty bearer tokens aren't valid
	if len(token) == 0 {
		return *claims, errcode.InvalidToken
	}

	newToken, parseErr := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(Config.JwtSecret), nil
	})
	if parseErr != nil {
		clog.Error("parse token error: %s", parseErr)
		return *claims, errcode.InvalidToken
	}
	if claims, ok := newToken.Claims.(*Claims); ok && newToken.Valid {
		return *claims, nil
	}
	return *claims, errcode.InvalidToken
}

func RefreshToken(token string) (string, *errcode.ErrorInfo) {
	claims, err := ParseToken(token)
	if err != nil {
		clog.Error("parse token error: %s", err)
		return "", errcode.InvalidToken
	}
	return GenerateToken(claims.UserInfo.Username, 0)
}
