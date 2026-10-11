package proxy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/gateway/internal/config"
)

type Gateway struct {
	handler http.Handler
	reads   *http.Transport
	writes  *http.Transport
}

func New(c config.Config, logger *slog.Logger) *Gateway {
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}
	reads := http.DefaultTransport.(*http.Transport).Clone()
	reads.Proxy = nil // Internal upstream traffic must not inherit an egress proxy.
	reads.DialContext = (&net.Dialer{Timeout: c.DialTimeout, KeepAlive: 30 * time.Second}).DialContext
	reads.TLSHandshakeTimeout = c.DialTimeout
	reads.ResponseHeaderTimeout = c.ResponseHeaderTimeout
	reads.DisableCompression = true // Preserve the upstream representation and headers.
	writes := reads.Clone()
	// A reused connection can cause net/http to replay a request carrying an
	// Idempotency-Key. Writes get a fresh HTTP/1 connection and no proxy retries.
	writes.DisableKeepAlives = true
	writes.Protocols = new(http.Protocols)
	writes.Protocols.SetHTTP1(true)
	readinessClient := &http.Client{
		Transport:     reads,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	newProxy := func(target *url.URL, upstream string) *httputil.ReverseProxy {
		unavailableMessage, timeoutMessage := "El backend no está disponible.", "El backend no respondió a tiempo."
		if upstream == "catalogo" {
			unavailableMessage, timeoutMessage = "Catálogo no está disponible.", "Catálogo no respondió a tiempo."
		}
		return &httputil.ReverseProxy{
			Transport: methodTransport{reads: reads, writes: writes},
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.SetURL(target)
				// The configured target is an origin; don't normalize paths or queries.
				pr.Out.URL.Path = pr.In.URL.Path
				pr.Out.URL.RawPath = pr.In.URL.RawPath
				pr.Out.URL.RawQuery = pr.In.URL.RawQuery
				pr.Out.URL.ForceQuery = pr.In.URL.ForceQuery
				pr.Out.Host = pr.In.Host
				pr.Out.Header.Set("X-Request-ID", pr.In.Header.Get("X-Request-ID"))
				pr.Out.Header.Del("X-User-ID")
				pr.Out.Header.Del("X-Role")
				pr.Out.Header.Del("X-User-Role")
				clientIP, scheme := forwardingIdentity(pr.In, c.TrustedProxies)
				pr.SetXForwarded()
				if clientIP != "" {
					pr.Out.Header.Set("X-Forwarded-For", clientIP)
				}
				pr.Out.Header.Set("X-Forwarded-Proto", scheme)
			},
			ModifyResponse: func(res *http.Response) error {
				res.Header.Set("X-Request-ID", res.Request.Header.Get("X-Request-ID"))
				return nil
			},
			ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				id := r.Header.Get("X-Request-ID")
				if errors.Is(r.Context().Err(), context.Canceled) {
					logger.Info("upstream_canceled", "request_id", id)
					return
				}
				status, code, message := http.StatusBadGateway, upstream+"_unavailable", unavailableMessage
				var timeout net.Error
				if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &timeout) && timeout.Timeout()) {
					status, code, message = http.StatusGatewayTimeout, upstream+"_timeout", timeoutMessage
				}
				logger.Warn("upstream_error", "request_id", id, "upstream", upstream, "status", status, "error", err.Error())
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Header().Set("X-Request-ID", id)
				w.WriteHeader(status)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "mensaje": message, "request_id": id})
			},
		}
	}
	backendProxy := newProxy(c.Backend, "backend")
	var catalogProxy *httputil.ReverseProxy
	if c.Catalogo != nil {
		catalogProxy = newProxy(c.Catalogo, "catalogo")
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requestID(r.Header.Get("X-Request-ID"))
		r = r.Clone(r.Context())
		r.Header.Set("X-Request-ID", id)
		w.Header().Set("X-Request-ID", id)
		started := time.Now()
		response := &responseWriter{ResponseWriter: w}
		defer func() {
			status := response.status
			if status == 0 {
				status = http.StatusOK
				if r.Context().Err() != nil {
					status = 499 // Logging only: client closed the request.
				}
			}
			logger.Info("request", "request_id", id, "method", r.Method,
				"path", r.URL.EscapedPath(), "status", status,
				"duration_ms", time.Since(started).Milliseconds(), "response_bytes", response.bytes)
		}()
		// Existing /health and /api/v1/health still belong to the backend.
		if r.URL.Path == "/_gateway/live" || r.URL.Path == "/_gateway/ready" {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				response.Header().Set("Allow", "GET, HEAD")
				response.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			response.Header().Set("Content-Type", "application/json; charset=utf-8")
			response.WriteHeader(http.StatusOK)
			if r.Method != http.MethodHead {
				_, _ = io.WriteString(response, "{\"status\":\"ok\",\"service\":\"gateway\"}\n")
			}
			return
		}
		if r.URL.Path == "/_gateway/dependencies" {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				response.Header().Set("Allow", "GET, HEAD")
				response.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), c.ReadinessTimeout)
			defer cancel()
			checks := dependencyChecks(ctx, readinessClient, c, id)
			status, state := http.StatusOK, "ok"
			for _, check := range checks {
				if check != "ok" {
					status, state = http.StatusServiceUnavailable, "not_ready"
				}
			}
			response.Header().Set("Content-Type", "application/json; charset=utf-8")
			response.WriteHeader(status)
			if r.Method != http.MethodHead {
				_ = json.NewEncoder(response).Encode(map[string]any{"status": state, "service": "gateway", "checks": checks})
			}
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), c.RequestTimeout)
		defer cancel()
		if catalogProxy != nil && isCatalogPath(r.URL.Path) {
			catalogProxy.ServeHTTP(response, r.WithContext(ctx))
		} else {
			backendProxy.ServeHTTP(response, r.WithContext(ctx))
		}
	})
	return &Gateway{handler: handler, reads: reads, writes: writes}
}

func isCatalogPath(path string) bool {
	for _, prefix := range []string{"/api/v1/productos", "/api/v1/admin/productos"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) { g.handler.ServeHTTP(w, r) }

func (g *Gateway) CloseIdleConnections() {
	g.reads.CloseIdleConnections()
	g.writes.CloseIdleConnections()
}

type methodTransport struct{ reads, writes *http.Transport }

func (t methodTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return t.reads.RoundTrip(r)
	default:
		return t.writes.RoundTrip(r)
	}
}

func requestID(value string) string {
	valid := len(value) > 0 && len(value) <= 128
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			valid = false
			break
		}
	}
	if valid {
		return value
	}
	var token [16]byte
	_, _ = rand.Read(token[:])
	return hex.EncodeToString(token[:])
}

func forwardingIdentity(r *http.Request, trusted []netip.Prefix) (string, string) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", scheme
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return "", scheme
	}
	peer = peer.Unmap()
	if !isTrusted(peer, trusted) {
		return peer.String(), scheme
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded == "http" || forwarded == "https" {
		scheme = forwarded
	}
	chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	// Walk only the trusted suffix. An untrusted client may prepend malformed
	// values; parsing those first would discard the client IP appended by ingress.
	for i := len(chain) - 1; i >= 0 && isTrusted(peer, trusted); i-- {
		addr, err := netip.ParseAddr(strings.TrimSpace(chain[i]))
		if err != nil {
			// Do not skip a malformed hop or trust any values to its left.
			break
		}
		peer = addr.Unmap()
	}
	return peer.String(), scheme
}

func isTrusted(addr netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// Unwrap preserves ResponseController support for flushing and protocol upgrades.
type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *responseWriter) WriteHeader(status int) {
	if w.status == 0 && (status >= 200 || status == http.StatusSwitchingProtocols) {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}
