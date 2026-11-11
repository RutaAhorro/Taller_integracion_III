package catalog

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ValidateCategoryURL comprueba que la URL sea HTTPS del sitio y cumpla las
// restricciones de path declaradas para categorías.
func (s Site) ValidateCategoryURL(rawURL string) error {
	return s.validate(rawURL, s.CategoryURL)
}

// ValidateProductURL comprueba que la URL sea HTTPS del sitio y cumpla las
// restricciones de path declaradas para fichas de producto.
func (s Site) ValidateProductURL(rawURL string) error {
	return s.validate(rawURL, s.ProductURL)
}

func (s Site) validate(rawURL string, rule URLRule) error {
	message := rule.Message
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return errors.New(message)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("%s", message)
	}
	if _, ok := s.hosts[strings.ToLower(parsed.Hostname())]; !ok {
		return fmt.Errorf("%s", message)
	}
	if rule.Prefix != "" && !strings.HasPrefix(parsed.Path, rule.Prefix) {
		return errors.New(message)
	}
	if rule.Suffix != "" && !strings.HasSuffix(strings.TrimRight(parsed.Path, "/"), rule.Suffix) {
		return errors.New(message)
	}
	if rule.MinSegments > 0 && len(nonEmptySegments(parsed.Path)) < rule.MinSegments {
		return errors.New(message)
	}
	return nil
}

func nonEmptySegments(path string) []string {
	var segments []string
	for _, segment := range strings.Split(path, "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	return segments
}
