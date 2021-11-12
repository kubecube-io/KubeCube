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

	"github.com/dgrijalva/jwt-go"
	"github.com/kubecube-io/kubecube/pkg/clog"
	"github.com/kubecube-io/kubecube/pkg/utils/constants"
	"k8s.io/api/authentication/v1beta1"
)

type AuthJwtImpl struct{}

func (a *AuthJwtImpl) Authentication(token string) (*v1beta1.UserInfo, error) {
	claims := &Claims{}

	// Empty bearer tokens aren't valid
	if len(token) == 0 {
		return nil, fmt.Errorf("invaild token")
	}

	newToken, parseErr := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(Config.JwtSecret), nil
	})
	if parseErr != nil {
		return nil, fmt.Errorf("parse token error, jwt secret: %v, token: %v, error: %v", Config.JwtSecret, token, parseErr)
	}
	if claims, ok := newToken.Claims.(*Claims); ok && newToken.Valid {
		return &claims.UserInfo, nil
	}
	return nil, fmt.Errorf("invaild token")
}

func (a *AuthJwtImpl) GenerateToken(user *v1beta1.UserInfo) (string, error) {
	return a.GenerateTokenWithExpired(user, constants.DefaultTokenExpireDuration)
}

func (a *AuthJwtImpl) GenerateTokenWithExpired(user *v1beta1.UserInfo, expireDuration int64) (string, error) {
	var tokenExpireDuration int64 = constants.DefaultTokenExpireDuration
	if Config.TokenExpireDuration > 0 {
		tokenExpireDuration = Config.TokenExpireDuration
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
			Issuer:    Config.JwtIssuer,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, signErr := token.SignedString([]byte(Config.JwtSecret))
	if signErr != nil {
		return "", fmt.Errorf("sign token with jwt secret error: %s", signErr)
	}
	clog.Debug("generate token success, new token is %v, secret is %v, issuer is %v", signedToken, Config.JwtSecret, Config.JwtIssuer)
	return signedToken, nil
}

func (a *AuthJwtImpl) RefreshToken(token string) (string, error) {
	claims, err := ParseToken(token)
	if err != nil {
		return "", fmt.Errorf("parse token error: %s", err)
	}
	return a.GenerateToken(&claims.UserInfo)
}
