package catalog

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ValidateCategoryURL comprueba que la URL sea HTTPS del sitio y cumpla las
// restricciones de path declaradas para categorías.
func (sitio Site) ValidateCategoryURL(urlCruda string) error {
	return sitio.validate(urlCruda, sitio.CategoryURL)
}

// ValidateProductURL comprueba que la URL sea HTTPS del sitio y cumpla las
// restricciones de path declaradas para fichas de producto.
func (sitio Site) ValidateProductURL(urlCruda string) error {
	return sitio.validate(urlCruda, sitio.ProductURL)
}

// ===== Validación Interna =====

// validate comprueba un esquema HTTPS, host permitido y segmentos de ruta.
func (sitio Site) validate(urlCruda string, regla URLRule) error {
	mensaje := regla.Message
	analizada, errorObtenido := url.Parse(urlCruda)
	if errorObtenido != nil {
		return errors.New(mensaje)
	}
	if analizada.Scheme != "https" {
		return fmt.Errorf("%s", mensaje)
	}
	if _, valido := sitio.hosts[strings.ToLower(analizada.Hostname())]; !valido {
		return fmt.Errorf("%s", mensaje)
	}
	if regla.Prefix != "" && !strings.HasPrefix(analizada.Path, regla.Prefix) {
		return errors.New(mensaje)
	}
	if regla.Suffix != "" && !strings.HasSuffix(strings.TrimRight(analizada.Path, "/"), regla.Suffix) {
		return errors.New(mensaje)
	}
	if regla.MinSegments > 0 && len(nonEmptySegments(analizada.Path)) < regla.MinSegments {
		return errors.New(mensaje)
	}
	return nil
}

// nonEmptySegments descompone la ruta en segmentos descartando componentes vacíos.
func nonEmptySegments(ruta string) []string {
	var segmentos []string
	for _, segmento := range strings.Split(ruta, "/") {
		if segmento != "" {
			segmentos = append(segmentos, segmento)
		}
	}
	return segmentos
}
