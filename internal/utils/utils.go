package utils //nolint:revive // package provides utility helpers used across modules

import (
	"net/url"

	"github.com/goccy/go-yaml"
)

// UnmarshalYAMLToMap parses YAML into a generic map[string]any.
// With go-yaml, timestamp scalars remain strings unless decoding into time.Time.
func UnmarshalYAMLToMap(src string) (map[string]any, error) {
	var m map[string]any

	err := yaml.Unmarshal([]byte(src), &m)
	if err != nil {
		return nil, err
	}

	if m == nil {
		m = map[string]any{}
	}

	return m, nil
}

// IsURL reports whether v is a URL with a non-empty scheme and host.
func IsURL(v string) bool {
	u, err := url.Parse(v)

	return err == nil && u.Scheme != "" && u.Host != ""
}
