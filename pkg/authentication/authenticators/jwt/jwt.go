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
	"fmt"
	"time"

	"github.com/golang-jwt/jwt"
	"k8s.io/api/authentication/v1beta1"

	"github.com/kubecube-io/kubecube/pkg/authentication"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"github.com/kubecube-io/kubecube/pkg/utils/env"
)

const BearerTokenPrefix = "Bearer"

var (
	Config      authentication.JwtConfig
	authJwtImpl AuthJwt
)

type AuthJwt struct {
	JwtSecret           string
	TokenExpireDuration int64
	JwtIssuer           string
}

type Claims struct {
	UserInfo v1beta1.UserInfo
	jwt.StandardClaims
}

func GetAuthJwtImpl() *AuthJwt {
	if authJwtImpl.JwtSecret != "" {
		return &authJwtImpl
	}
	return &AuthJwt{
		JwtSecret:           env.JwtSecret(),
		TokenExpireDuration: Config.TokenExpireDuration,
		JwtIssuer:           Config.JwtIssuer,
	}
}

func (a *AuthJwt) GenerateToken(user *v1beta1.UserInfo) (string, error) {
	return a.GenerateTokenWithExpired(user, constants.DefaultTokenExpireDuration)
}

func (a *AuthJwt) GenerateTokenWithExpired(user *v1beta1.UserInfo, expireDuration int64) (string, error) {
	var tokenExpireDuration int64 = constants.DefaultTokenExpireDuration
	if a.TokenExpireDuration > 0 {
		tokenExpireDuration = a.TokenExpireDuration
	}
	if expireDuration > 0 {
		tokenExpireDuration = expireDuration
	}

	claims := Claims{
		UserInfo: v1beta1.UserInfo{
			Username: user.Username,
			Groups:   []string{constants.KubeCube},
		},
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Unix() + tokenExpireDuration,
			Issuer:    a.JwtIssuer,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, signErr := token.SignedString([]byte(a.JwtSecret))
	if signErr != nil {
		return "", fmt.Errorf("sign token with jwt secret error: %s", signErr)
	}
	clog.Debug("generate token success, new token is %v, secret is %v, issuer is %v", signedToken, a.JwtSecret, a.JwtIssuer)
	return signedToken, nil
}

func (a *AuthJwt) Authentication(token string) (user *v1beta1.UserInfo, err error) {
	claims := &Claims{}

	// Empty bearer tokens aren't valid
	if len(token) == 0 {
		return nil, fmt.Errorf("invaild token")
	}

	newToken, parseErr := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(a.JwtSecret), nil
	})
	if parseErr != nil {
		return nil, fmt.Errorf("parse token error, jwt secret: %v, token: %v, error: %v", a.JwtSecret, token, parseErr)
	}
	if claims, ok := newToken.Claims.(*Claims); ok && newToken.Valid {
		return &claims.UserInfo, nil
	}
	return nil, fmt.Errorf("invaild token")
}

func (a *AuthJwt) RefreshToken(token string) (*v1beta1.UserInfo, string, error) {
	userInfo, err := a.Authentication(token)
	if err != nil {
		return nil, "", err
	}

	newToken, err := a.GenerateToken(userInfo)
	if err != nil {
		return nil, "", err
	}

	return userInfo, newToken, nil
}
