package namespace

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	PrincipalPrefix = "$ns/"
	TopicPrefix     = "$iothub/ns/"
)

var nameRegexp = regexp.MustCompile(`^[0-9A-Za-z_-]+$`)

func ValidName(ns string) bool {
	return nameRegexp.MatchString(ns)
}

func Principal(ns string) string {
	return PrincipalPrefix + ns
}

func ParsePrincipal(principal string) (string, bool) {
	if !strings.HasPrefix(principal, PrincipalPrefix) {
		return "", false
	}
	ns := strings.TrimPrefix(principal, PrincipalPrefix)
	return ns, ValidName(ns)
}

func TopicPrefixOf(ns string) string {
	return TopicPrefix + ns + "/"
}

func TopicPresence(ns, thingId string) string {
	return fmt.Sprintf("%sthings/%s/presence", TopicPrefixOf(ns), thingId)
}

func TopicPresenceEvent(ns, thingId string) string {
	return fmt.Sprintf("%sevents/things/%s/presence", TopicPrefixOf(ns), thingId)
}

func TopicShadowUpdated(ns, thingId string) string {
	return fmt.Sprintf("%sthings/%s/shadows/name/default/update/documents", TopicPrefixOf(ns), thingId)
}
