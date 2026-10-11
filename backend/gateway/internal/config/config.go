package config

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr            string
	Backend               *url.URL
	Catalogo              *url.URL
	RequestTimeout        time.Duration
	DialTimeout           time.Duration
	ResponseHeaderTimeout time.Duration
	ReadHeaderTimeout     time.Duration
	IdleTimeout           time.Duration
	ShutdownTimeout       time.Duration
	ReadinessTimeout      time.Duration
	TrustedProxies        []netip.Prefix
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

	c.Backend, err = parseOrigin("BACKEND_URL", getenv("BACKEND_URL"))
	if err != nil {
		return c, err
	}
	// Empty is an explicit legacy mode for rollback with a pre-cutover API image.
	if value := getenv("CATALOGO_URL"); value != "" {
		c.Catalogo, err = parseOrigin("CATALOGO_URL", value)
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
