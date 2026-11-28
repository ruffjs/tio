package nats

import (
	"errors"
	"strings"
	"unicode"
)

func validateMqttLevels(levels []string, allowHash bool, allowPlus bool) error {
	for i, level := range levels {
		if level == "" {
			return errors.New("empty level")
		}

		for _, r := range level {
			if unicode.IsSpace(r) {
				return errors.New("whitespace in level")
			}
		}

		if strings.Contains(level, ".") {
			return errors.New("dot in level")
		}

		if strings.ContainsAny(level, "*>") {
			return errors.New("nats wildcard in mqtt level")
		}

		isLast := i == len(levels)-1

		if strings.Contains(level, "#") {
			if !allowHash {
				return errors.New("wildcard # not allowed")
			}
			if level != "#" || !isLast {
				return errors.New("# must be the last complete level")
			}
		}

		if strings.Contains(level, "+") {
			if !allowPlus {
				return errors.New("wildcard + not allowed")
			}
			if level != "+" {
				return errors.New("+ must occupy a complete level")
			}
		}
	}

	return nil
}

func parseMqttTopic(topic string, allowHash bool, allowPlus bool) ([]string, error) {
	if topic == "" {
		return nil, errors.New("empty topic")
	}
	if strings.HasPrefix(topic, "/") {
		return nil, errors.New("leading slash")
	}
	if strings.HasSuffix(topic, "/") {
		return nil, errors.New("trailing slash")
	}

	levels := strings.Split(topic, "/")
	if err := validateMqttLevels(levels, allowHash, allowPlus); err != nil {
		return nil, err
	}

	return levels, nil
}

func MqttSubscriptionToNatsSubjects(filter string) ([]string, error) {
	levels, err := parseMqttTopic(filter, true, true)
	if err != nil {
		return nil, err
	}

	if filter == "#" {
		return []string{">"}, nil
	}

	if strings.HasSuffix(filter, "/#") {
		parentLevels := levels[:len(levels)-1]
		for i, l := range parentLevels {
			if l == "+" {
				parentLevels[i] = "*"
			}
		}
		parentSubject := strings.Join(parentLevels, ".")
		return []string{parentSubject, parentSubject + ".>"}, nil
	}

	result := make([]string, len(levels))
	for i, level := range levels {
		if level == "+" {
			result[i] = "*"
		} else {
			result[i] = level
		}
	}

	return []string{strings.Join(result, ".")}, nil
}

func MqttPublishTopicToNatsSubject(topic string) (string, error) {
	levels, err := parseMqttTopic(topic, false, false)
	if err != nil {
		return "", err
	}

	return strings.Join(levels, "."), nil
}

func validateNatsLevels(levels []string) error {
	for _, level := range levels {
		if level == "" {
			return errors.New("empty level")
		}
		if strings.Contains(level, "/") {
			return errors.New("slash in nats level")
		}
		for _, r := range level {
			if unicode.IsSpace(r) {
				return errors.New("whitespace in nats level")
			}
		}
		if strings.ContainsAny(level, "*>") {
			return errors.New("nats wildcard in concrete subject")
		}
	}
	return nil
}

func NatsSubjectToMqttTopic(subject string) (string, error) {
	if subject == "" {
		return "", errors.New("empty subject")
	}
	if strings.HasPrefix(subject, ".") {
		return "", errors.New("leading dot")
	}
	if strings.HasSuffix(subject, ".") {
		return "", errors.New("trailing dot")
	}

	levels := strings.Split(subject, ".")
	if err := validateNatsLevels(levels); err != nil {
		return "", err
	}

	return strings.Join(levels, "/"), nil
}
