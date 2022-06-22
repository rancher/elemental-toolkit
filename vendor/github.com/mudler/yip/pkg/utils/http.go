package utils

import "net/url"

func IsUrl(s string) bool {
	url, err := url.Parse(s)
	if err != nil || url.Scheme == "" {
		return false
	}
	return true
}
