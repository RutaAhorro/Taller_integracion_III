package catalog

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ValidateCategoryURL comprueba que la URL sea HTTPS del sitio y cumpla las
// restricciones de path declaradas para categorías.
func (sitio Site) ValidateCategoryURL(rawURL string) error {
	return sitio.validate(rawURL, sitio.CategoryURL)
}

// ValidateProductURL comprueba que la URL sea HTTPS del sitio y cumpla las
// restricciones de path declaradas para fichas de producto.
func (sitio Site) ValidateProductURL(rawURL string) error {
	return sitio.validate(rawURL, sitio.ProductURL)
}

func (sitio Site) validate(rawURL string, rule URLRule) error {
	message := rule.Message
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return errors.New(message)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("%s", message)
	}
	if _, ok := sitio.hosts[strings.ToLower(parsed.Hostname())]; !ok {
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
