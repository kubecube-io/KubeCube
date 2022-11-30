package request

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode"
)

const (
	FieldManager = "fieldManager"
)

func AddFieldManager(req *http.Request, username string) error {
	for _, r := range username {
		if !unicode.IsPrint(r) {
			return fmt.Errorf("username not printable")
		}
	}

	username = "cube-" + username

	if len(username) > 128 {
		return fmt.Errorf("username should not be longer than 128")
	}

	return AddQuery(req, FieldManager, username)
}

func AddQuery(req *http.Request, key, value string) error {
	query, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		return err
	}
	query.Set(key, value)
	newQueryString := query.Encode()
	req.URL.RawQuery = newQueryString
	path := strings.Split(req.RequestURI, "?")[0]
	req.RequestURI = path + "?" + newQueryString

	return nil
}
