package resource

import (
	"fmt"
	"sort"
	"strings"
)

type Tags map[string]string

func (tags *Tags) String() string {
	if len(*tags) == 0 {
		return ""
	}
	pairs := make([]string, 0, len(*tags))
	for key, value := range *tags {
		// handle colons, like aws:cloudformation:stack-name key
		safeKey := strings.ReplaceAll(key, ":", "_")
		safeValue := strings.ReplaceAll(value, ":", "_")

		pairs = append(pairs, fmt.Sprintf("%s_%s", safeKey, safeValue))
	}
	sort.Strings(pairs)
	return strings.Join(pairs, "-")
}

func (tags *Tags) Set(value string) error {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid key=value pair %q", value)
	}
	key := strings.TrimSpace(parts[0])
	val := strings.TrimSpace(parts[1])
	if key == "" || val == "" {
		return fmt.Errorf("invalid key=value pair %q", value)
	}
	(*tags)[key] = val
	return nil
}
