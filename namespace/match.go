package namespace

import (
	"fmt"

	"ruff.io/tio/config"
	"ruff.io/tio/shadow"
)

func MatchNamespaces(namespaces []config.Namespace, tags shadow.TagsValue) []string {
	matches := make([]string, 0)
	for _, ns := range namespaces {
		if !ValidName(ns.Name) || len(ns.Tags) == 0 {
			continue
		}
		if matchTags(ns.Tags, tags) {
			matches = append(matches, ns.Name)
		}
	}
	return matches
}

func matchTags(want map[string]string, got shadow.TagsValue) bool {
	for k, v := range want {
		gotValue, ok := got[k]
		if !ok {
			return false
		}
		if fmt.Sprint(gotValue) != v {
			return false
		}
	}
	return true
}

func NamespaceForUser(namespaces []config.Namespace, username, password string) (string, bool) {
	for _, ns := range namespaces {
		if !ValidName(ns.Name) {
			continue
		}
		for _, u := range ns.Users {
			if u.Name == username && u.Password == password {
				return ns.Name, true
			}
		}
	}
	return "", false
}

func Configured(namespaces []config.Namespace, name string) bool {
	for _, ns := range namespaces {
		if ns.Name == name && ValidName(ns.Name) {
			return true
		}
	}
	return false
}
