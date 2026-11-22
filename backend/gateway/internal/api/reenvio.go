// reenvio.go decide qué datos del cliente viajan al upstream: un X-Request-ID
// válido y la IP/esquema reales, aceptando X-Forwarded-* sólo de proxies fiables.
package api

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ===== Identificador de petición =====

const (
	// largoMaximoIDPeticion acota el ID que se copia a logs y respuestas.
	largoMaximoIDPeticion = 128
	// bytesIDGenerado da 32 caracteres hexadecimales al ID nuevo.
	bytesIDGenerado = 16
)

// normalizarIDPeticion conserva el ID del cliente si es corto y sólo usa
// [A-Za-z0-9._-]; si no, genera uno aleatorio para no inyectar texto en logs.
func normalizarIDPeticion(valor string) string {
	if idPeticionValido(valor) {
		return valor
	}
	var aleatorio [bytesIDGenerado]byte
	_, _ = rand.Read(aleatorio[:])
	return hex.EncodeToString(aleatorio[:])
}

// idPeticionValido comprueba largo y alfabeto permitido.
func idPeticionValido(valor string) bool {
	if valor == "" || len(valor) > largoMaximoIDPeticion {
		return false
	}
	for _, caracter := range valor {
		alfanumerico := caracter >= 'a' && caracter <= 'z' || caracter >= 'A' && caracter <= 'Z' || caracter >= '0' && caracter <= '9'
		if !alfanumerico && caracter != '-' && caracter != '_' && caracter != '.' {
			return false
		}
	}
	return true
}

// ===== X-Forwarded-* =====

// identidadReenvio devuelve la IP del cliente y el esquema. Con un par no
// fiable se ignoran sus cabeceras; con uno fiable se recorre X-Forwarded-For
// de derecha a izquierda mientras cada salto siga siendo fiable.
func identidadReenvio(peticion *http.Request, fiables []netip.Prefix) (string, string) {
	esquema := "http"
	if peticion.TLS != nil {
		esquema = "https"
	}
	host, _, errorHost := net.SplitHostPort(peticion.RemoteAddr)
	if errorHost != nil {
		return "", esquema
	}
	par, errorPar := netip.ParseAddr(host)
	if errorPar != nil {
		return "", esquema
	}
	par = par.Unmap()
	if !esFiable(par, fiables) {
		return par.String(), esquema
	}
	// Sólo valores exactos: cualquier otro texto se descarta para no reenviar
	// un esquema que el upstream no sabría interpretar.
	if reenviado := peticion.Header.Get("X-Forwarded-Proto"); reenviado == "http" || reenviado == "https" {
		esquema = reenviado
	}
	cadena := strings.Split(peticion.Header.Get("X-Forwarded-For"), ",")
	// Un cliente no fiable puede anteponer valores mal formados; leerlos
	// primero haría perder la IP que agregó el ingress.
	for indice := len(cadena) - 1; indice >= 0 && esFiable(par, fiables); indice-- {
		direccion, errorDireccion := netip.ParseAddr(strings.TrimSpace(cadena[indice]))
		if errorDireccion != nil {
			// No se salta un valor mal formado ni se confía en lo que hay a su izquierda.
			break
		}
		par = direccion.Unmap()
	}
	return par.String(), esquema
}

// esFiable indica si la dirección cae en alguna red de GATEWAY_TRUSTED_PROXIES.
func esFiable(direccion netip.Addr, redes []netip.Prefix) bool {
	for _, red := range redes {
		if red.Contains(direccion) {
			return true
		}
	}
	return false
}
