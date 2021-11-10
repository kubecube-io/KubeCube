package authenticators

import (
	"k8s.io/api/authentication/v1beta1"
)

type AuthNManager interface {
	Authentication(token string) (*v1beta1.UserInfo, error)
	GenerateToken(user *v1beta1.UserInfo) (string, error)
	GenerateTokenWithExpired(user *v1beta1.UserInfo, expireDuration int64) (string, error)
	RefreshToken(token string) (string, error)
}
