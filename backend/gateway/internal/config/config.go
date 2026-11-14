package config

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Config struct {
	ListenAddr            string
	Backend               *url.URL
	Catalogo              *url.URL
	Identidad             *url.URL
	Scraping              *url.URL
	Rutas                 *url.URL
	IA                    *url.URL
	RequestTimeout        time.Duration
	DialTimeout           time.Duration
	ResponseHeaderTimeout time.Duration
	ReadHeaderTimeout     time.Duration
	IdleTimeout           time.Duration
	ShutdownTimeout       time.Duration
	ReadinessTimeout      time.Duration
	KeepAliveTimeout      time.Duration
	TrustedProxies        []netip.Prefix
	CatalogPrefixes       []string
	IdentityPrefixes      []string
	RoutesPrefixes        []string
	ScrapingPrefixes      []string
	IAPrefixes            []string
}

// Load reads configuration without credentials or database access.
func Load(getenv func(string) string) (Config, error) {
	c := Config{ListenAddr: ":8082"}
	if value := getenv("GATEWAY_LISTEN_ADDR"); value != "" {
		c.ListenAddr = value
	}
	_, port, err := net.SplitHostPort(c.ListenAddr)
	if err != nil {
		return c, fmt.Errorf("GATEWAY_LISTEN_ADDR must be a host:port address")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return c, fmt.Errorf("GATEWAY_LISTEN_ADDR must have a port between 1 and 65535")
	}

	if value := getenv("SCRAPING_URL"); value != "" {
		c.Scraping, err = parseOrigin("SCRAPING_URL", value)
		if err != nil {
			return c, err
		}
	}
	if value := getenv("BACKEND_URL"); value != "" || c.Scraping == nil {
		c.Backend, err = parseOrigin("BACKEND_URL", value)
		if err != nil {
			return c, err
		}
	}
	// Empty is an explicit legacy mode for rollback with a pre-cutover API image.
	if value := getenv("CATALOGO_URL"); value != "" {
		c.Catalogo, err = parseOrigin("CATALOGO_URL", value)
		if err != nil {
			return c, err
		}
	}

	// Vacío sólo permite rollback a una API histórica que todavía aloje auth.
	if value := getenv("IDENTIDAD_URL"); value != "" {
		c.Identidad, err = parseOrigin("IDENTIDAD_URL", value)
		if err != nil {
			return c, err
		}
	}

	// SUP-240: motor de rutas, dueño de listas. Vacío deja /api/v1/listas sin enrutar.
	if value := getenv("RUTAS_URL"); value != "" {
		c.Rutas, err = parseOrigin("RUTAS_URL", value)
		if err != nil {
			return c, err
		}
	}

	// SUP-278: IA conversacional, dueña de chat. Vacío deja /api/v1/chat sin enrutar.
	if value := getenv("IA_URL"); value != "" {
		c.IA, err = parseOrigin("IA_URL", value)
		if err != nil {
			return c, err
		}
	}

	for _, item := range []struct {
		name     string
		value    *time.Duration
		fallback time.Duration
	}{
		{"GATEWAY_REQUEST_TIMEOUT", &c.RequestTimeout, 30 * time.Second},
		{"GATEWAY_DIAL_TIMEOUT", &c.DialTimeout, 5 * time.Second},
		{"GATEWAY_RESPONSE_HEADER_TIMEOUT", &c.ResponseHeaderTimeout, 10 * time.Second},
		{"GATEWAY_READ_HEADER_TIMEOUT", &c.ReadHeaderTimeout, 5 * time.Second},
		{"GATEWAY_IDLE_TIMEOUT", &c.IdleTimeout, 60 * time.Second},
		{"GATEWAY_SHUTDOWN_TIMEOUT", &c.ShutdownTimeout, 10 * time.Second},
		{"GATEWAY_READINESS_TIMEOUT", &c.ReadinessTimeout, 2 * time.Second},
		{"GATEWAY_KEEPALIVE_TIMEOUT", &c.KeepAliveTimeout, 30 * time.Second},
	} {
		*item.value = item.fallback
		if value := getenv(item.name); value != "" {
			duration, err := time.ParseDuration(value)
			if err != nil || duration <= 0 {
				return c, fmt.Errorf("%s must be a positive duration, for example 5s", item.name)
			}
			*item.value = duration
		}
	}

	// Route prefixes are validated at startup: a malformed value would leave
	// routes unreachable while the Gateway still reports itself healthy.
	seenPrefixes := map[string]string{}
	for _, item := range []struct {
		name     string
		value    *[]string
		fallback []string
	}{
		{"GATEWAY_IDENTIDAD_PREFIXES", &c.IdentityPrefixes, []string{"/api/v1/auth", "/api/v1/admin/usuarios"}},
		{"GATEWAY_CATALOGO_PREFIXES", &c.CatalogPrefixes, []string{"/api/v1/productos", "/api/v1/admin/productos"}},
		{"GATEWAY_RUTAS_PREFIXES", &c.RoutesPrefixes, []string{"/api/v1/listas"}},
		{"GATEWAY_SCRAPING_PREFIXES", &c.ScrapingPrefixes, []string{"/api/v1/scraper", "/swagger"}},
		{"GATEWAY_IA_PREFIXES", &c.IAPrefixes, []string{"/api/v1/chat"}},
	} {
		prefixes, err := parsePrefixList(item.name, getenv(item.name), item.fallback)
		if err != nil {
			return c, err
		}
		for _, prefix := range prefixes {
			if owner, duplicated := seenPrefixes[prefix]; duplicated {
				return c, fmt.Errorf("%s: prefix %q is already assigned by %s", item.name, prefix, owner)
			}
			seenPrefixes[prefix] = item.name
		}
		*item.value = prefixes
	}

	if value := getenv("GATEWAY_TRUSTED_PROXIES"); value != "" {
		for _, entry := range strings.Split(value, ",") {
			entry = strings.TrimSpace(entry)
			prefix, err := netip.ParsePrefix(entry)
			if err != nil {
				addr, addrErr := netip.ParseAddr(entry)
				if addrErr != nil {
					return c, fmt.Errorf("GATEWAY_TRUSTED_PROXIES must contain IP addresses or CIDRs")
				}
				prefix = netip.PrefixFrom(addr, addr.BitLen())
			}
			c.TrustedProxies = append(c.TrustedProxies, prefix.Masked())
		}
	}
	return c, nil
}

func parseOrigin(name, value string) (*url.URL, error) {
	target, err := url.Parse(value)
	if err != nil || target == nil || target.Hostname() == "" ||
		(target.Scheme != "http" && target.Scheme != "https") ||
		target.User != nil || target.Opaque != "" ||
		(target.Path != "" && target.Path != "/") ||
		target.RawQuery != "" || target.ForceQuery || target.Fragment != "" ||
		strings.ContainsAny(target.Host, " \t\r\n") {
		return nil, fmt.Errorf("%s must be an http(s) origin without credentials, path, query or fragment", name)
	}
	if port := target.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("%s port must be between 1 and 65535", name)
		}
	}
	return target, nil
}

// parsePrefixList reads a comma-separated list of path prefixes. An empty
// value keeps the defaults. Each entry must be an absolute path such as
// /api/v1/productos: no scheme, host, query, fragment, percent-encoding, dot
// segments or inner whitespace. A trailing slash is trimmed, so "/api/v1/x/"
// behaves as "/api/v1/x" and still matches its child routes. Empty entries,
// the bare root "/" and duplicates are rejected so startup fails fast.
func parsePrefixList(name, value string, fallback []string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return append([]string(nil), fallback...), nil
	}
	entries := strings.Split(value, ",")
	prefixes := make([]string, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		prefix := strings.TrimSpace(entry)
		invalid := func(reason string) error {
			return fmt.Errorf("%s: invalid prefix %q: %s", name, prefix, reason)
		}
		switch {
		case prefix == "":
			return nil, fmt.Errorf("%s: empty entry in %q", name, value)
		case strings.Contains(prefix, "://"):
			return nil, invalid("must be a path, not a URL")
		case !strings.HasPrefix(prefix, "/") || strings.HasPrefix(prefix, "//"):
			return nil, invalid("must start with a single /")
		case strings.ContainsAny(prefix, "?#"):
			return nil, invalid("must not contain query or fragment")
		case strings.Contains(prefix, "%"):
			return nil, invalid("must not be percent-encoded")
		case strings.IndexFunc(prefix, unicode.IsSpace) >= 0:
			return nil, invalid("must not contain whitespace")
		}
		prefix = strings.TrimRight(prefix, "/")
		if prefix == "" {
			return nil, invalid("the root / would capture every route")
		}
		for _, segment := range strings.Split(prefix[1:], "/") {
			if segment == "" || segment == "." || segment == ".." {
				return nil, invalid("must not contain empty or dot segments")
			}
		}
		if seen[prefix] {
			return nil, invalid("duplicated")
		}
		seen[prefix] = true
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}
