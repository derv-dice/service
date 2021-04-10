package service

import "regexp"

var rxName = regexp.MustCompile(`(?i)^([a-z])([a-z0-9_\-]){0,62}$`)

func validName(name string) bool {
	return rxName.MatchString(name)
}
