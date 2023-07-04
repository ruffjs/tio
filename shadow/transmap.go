package shadow

import "strings"

func transMap(m map[string]any, p map[string]string) map[string]any {
	if len(p) == 0 {
		return m
	}
	resM := make(map[string]any)

	for key, value := range p {
		propValue := getPropFromMap(m, key)
		// omit nil value
		if propValue != nil {
			resM[value] = propValue
		}
	}

	return resM
}

func getPropFromMap(m map[string]any, key string) any {
	keys := strings.Split(key, ".")

	cur := m
	lastIndex := len(keys) - 1
	for i, k := range keys {
		if value, ok := cur[k]; ok {
			if i == lastIndex {
				return value
			}
			switch v := value.(type) {
			case map[string]any:
				cur = v
			default:
				return nil
			}
		} else {
			return nil
		}
	}

	return nil
}
