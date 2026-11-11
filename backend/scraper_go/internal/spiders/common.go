package spiders

import (
	"os"
	"path/filepath"
	"strings"
)

func readCategoryURLs(file string) []string {
	body, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var urls []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		urls = append(urls, line)
	}
	return urls
}

func appendCategoryURL(file, rawURL string, validate func(string) error) error {
	if err := validate(rawURL); err != nil {
		return err
	}
	for _, existing := range readCategoryURLs(file) {
		if existing == rawURL {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	handle, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer handle.Close()
	_, err = handle.WriteString(rawURL + "\n")
	return err
}

func envCategoryURLs(name string) []string {
	var urls []string
	for _, raw := range strings.Split(os.Getenv(name), ",") {
		if value := strings.TrimSpace(raw); value != "" {
			urls = append(urls, value)
		}
	}
	return urls
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
