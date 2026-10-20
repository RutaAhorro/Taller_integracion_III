package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	// ErrSesionInactiva indica que Identidad rechazó la cookie porque la cuenta
	// fue desactivada, eliminada o perdió la sesión.
	ErrSesionInactiva = errors.New("sesión inactiva")
	// ErrSesionNoDisponible indica un fallo al consultar a Identidad. El
	// llamador debe fallar cerrado, nunca aceptar el rol del JWT.
	ErrSesionNoDisponible = errors.New("validador de sesión no disponible")
)

// Session es la identidad vigente devuelta por Identidad.
type Session struct {
	UserID string
	Rol    string
}

// SessionValidator revalida la cookie contra el dueño de api.usuarios.
type SessionValidator interface {
	Validate(ctx context.Context, cookieHeader string) (*Session, error)
}

// HTTPValidator consulta GET /api/v1/auth/me en Identidad. Identidad revalida
// la cuenta en api.usuarios y devuelve el rol vigente, no el claim del JWT.
type HTTPValidator struct {
	endpoint string
	client   *http.Client
}

// NewHTTPValidator valida que IDENTIDAD_URL sea un origen HTTP(S) exacto.
func NewHTTPValidator(origin string) (*HTTPValidator, error) {
	origin = strings.TrimSpace(strings.TrimRight(origin, "/"))
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Hostname() == "" || parsed.User != nil || parsed.Path != "" ||
		parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" ||
		strings.ContainsAny(origin, "*?#\\ \t\r\n") {
		return nil, errors.New("IDENTIDAD_URL debe ser un origen HTTP(S) exacto, sin ruta, credenciales ni query")
	}
	return &HTTPValidator{
		endpoint: origin + "/api/v1/auth/me",
		client:   &http.Client{Timeout: 2 * time.Second},
	}, nil
}

// Validate no devuelve el cuerpo de Identidad: sólo la identidad vigente.
func (v *HTTPValidator) Validate(ctx context.Context, cookieHeader string) (*Session, error) {
	if v == nil || v.endpoint == "" {
		return nil, ErrSesionNoDisponible
	}
	if strings.TrimSpace(cookieHeader) == "" {
		return nil, ErrSesionInactiva
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSesionNoDisponible, err)
	}
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSesionNoDisponible, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrSesionInactiva
	default:
		return nil, fmt.Errorf("%w: Identidad respondió %d", ErrSesionNoDisponible, resp.StatusCode)
	}

	var body struct {
		UserID string `json:"user_id"`
		Rol    string `json:"rol"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("%w: respuesta inválida de Identidad", ErrSesionNoDisponible)
	}
	body.UserID = strings.TrimSpace(body.UserID)
	body.Rol = strings.TrimSpace(body.Rol)
	if body.UserID == "" || body.Rol == "" {
		return nil, fmt.Errorf("%w: respuesta de Identidad incompleta", ErrSesionNoDisponible)
	}
	return &Session{UserID: body.UserID, Rol: body.Rol}, nil
}
