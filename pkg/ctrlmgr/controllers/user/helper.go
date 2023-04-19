package controllers

import (
	userv1 "github.com/kubecube-io/kubecube/pkg/apis/user/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

// calculateUserRelationShip calculates user relationship by scope bindings.
func calculateUserRelationShip(user *userv1.User) {

}

func ignoreAlreadyExistErr(err error) error {
	if errors.IsAlreadyExists(err) {
		return nil
	}
	return err
}
