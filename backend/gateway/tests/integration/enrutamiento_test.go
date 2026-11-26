// enrutamiento_test.go comprueba que cada prefijo llega a su dominio, que las
// rutas sin dueño responden 404 y que la caída de un dominio no afecta a otro.
package integration_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// metodosEnrutamiento son los métodos con los que se prueba cada ruta.
var metodosEnrutamiento = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}

// ===== Enrutamiento completo =====

// TestEnrutamientoPorDominio arranca los cinco dominios y verifica dueño,
// prefijos por segmento y 404 propio para rutas sin dueño.
func TestEnrutamientoPorDominio(prueba *testing.T) {
	control := servidorEco(prueba, "scraping")
	pasarela := pasarelaPara(prueba, control.URL, map[string]string{
		"IDENTIDAD_URL": servidorEco(prueba, "identidad").URL,
		"CATALOGO_URL":  servidorEco(prueba, "catalogo").URL,
		"RUTAS_URL":     servidorEco(prueba, "rutas").URL,
		"IA_URL":        servidorEco(prueba, "ia").URL,
	}, nil)
	comprobarEnrutamiento(prueba, pasarela, []casoEnrutamiento{
		// Identidad.
		{"/api/v1/auth", "identidad"}, {"/api/v1/auth/login", "identidad"}, {"/api/v1/auth/a%2Fb", "identidad"},
		{"/api/v1/admin/usuarios", "identidad"}, {"/api/v1/admin/usuarios/", "identidad"},
		{"/api/v1/admin/usuarios/2f1c1c1e-0000-4000-8000-000000000000", "identidad"},
		// SUP-239: direcciones del usuario. El prefijo es la ruta completa, no /api/v1/usuarios.
		{"/api/v1/usuarios/me/direcciones", "identidad"}, {"/api/v1/usuarios/me/direcciones/", "identidad"},
		{"/api/v1/usuarios/me/direcciones/2f1c1c1e-0000-4000-8000-000000000000", "identidad"},
		// Catálogo.
		{"/api/v1/productos", "catalogo"}, {"/api/v1/productos/", "catalogo"}, {"/api/v1/productos/a%2Fb", "catalogo"},
		{"/api/v1/admin/productos", "catalogo"}, {"/api/v1/admin/productos/", "catalogo"},
		// Motor de rutas.
		{"/api/v1/listas", "rutas"}, {"/api/v1/listas/", "rutas"}, {"/api/v1/listas/0b9d0c1e-1111-4111-8111-111111111111", "rutas"},
		{"/api/v1/rutas", "rutas"}, {"/api/v1/rutas/optimizar", "rutas"},
		// IA conversacional.
		{"/api/v1/chat", "ia"}, {"/api/v1/chat/", "ia"},
		// Control de scraping, incluida su salud.
		{"/api/v1/scraper/trabajos", "scraping"}, {"/api/v1/scraper/a%2Fb", "scraping"},
		{"/api/v1/scraper/trabajos/123/productos", "scraping"}, {"/swagger/doc.json", "scraping"},
		{"/", "scraping"}, {"/health", "scraping"}, {"/api/v1/health", "scraping"},
		// Sin dueño: prefijos vecinos y sondas internas de cada servicio.
		{"/api/v1/authentication", ""}, {"/api/v1/auth-falso", ""}, {"/api/v1/admin/usuarios-falso", ""},
		{"/api/v1/usuarios/me/direccionesx", ""}, {"/api/v1/usuarios/me", ""}, {"/api/v1/usuarios", ""}, {"/api/v1/usuarios/me/direcciones-falsas", ""},
		{"/api/v1/productos-falso", ""}, {"/api/v1/admin/productos-falso", ""},
		{"/api/v1/listas-falsas", ""}, {"/api/v1/listasx", ""}, {"/api/v1/rutas-falsas", ""}, {"/api/v1/rutasx", ""},
		{"/api/v1/chat-falso", ""}, {"/api/v1/chatx", ""}, {"/api/v1/scrapertools", ""},
		{"/_identidad/ready", ""}, {"/_catalogo/ready", ""}, {"/_rutas/ready", ""},
		{"/_scraping/ready", ""}, {"/_scraping/live", ""}, {"/api/v1/inexistente", ""},
	}, metodosEnrutamiento)
}

// TestDominiosOpcionalesSinOrigenResponden404 verifica que sin su variable
// un dominio no se enruta ni cae en Control.
func TestDominiosOpcionalesSinOrigenResponden404(prueba *testing.T) {
	control := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
		prueba.Errorf("la ruta no debe caer en Control: %s", peticion.URL.Path)
	}))
	defer control.Close()
	pasarela := pasarelaPara(prueba, control.URL, nil, nil)
	comprobarEstados(prueba, pasarela, []casoEstado{
		{ruta: "/api/v1/auth/me", estado: http.StatusNotFound},
		{ruta: "/api/v1/productos", estado: http.StatusNotFound},
		{ruta: "/api/v1/listas", estado: http.StatusNotFound},
		{ruta: "/api/v1/rutas/optimizar", estado: http.StatusNotFound},
		{ruta: "/api/v1/chat", estado: http.StatusNotFound},
	})
	respuesta, errorPeticion := http.Head(pasarela.URL + "/api/v1/inexistente")
	if errorPeticion != nil {
		prueba.Fatal(errorPeticion)
	}
	if cuerpo := cuerpoDe(prueba, respuesta); respuesta.StatusCode != http.StatusNotFound || len(cuerpo) != 0 {
		prueba.Fatalf("HEAD sin dueño debe ser 404 sin cuerpo: %d %q", respuesta.StatusCode, cuerpo)
	}
}

// ===== Aislamiento de fallos =====

// TestCaidaDeUnDominioNoAfectaALosDemas apaga cada dominio por turno y
// verifica su 502 con mensaje propio mientras el resto sigue respondiendo.
func TestCaidaDeUnDominioNoAfectaALosDemas(prueba *testing.T) {
	rutaPorDominio := map[string]string{
		"identidad": "/api/v1/auth/me", "catalogo": "/api/v1/productos", "rutas": "/api/v1/listas",
		"ia": "/api/v1/chat", "scraping": "/api/v1/scraper/trabajos",
	}
	for caido := range rutaPorDominio {
		prueba.Run(caido, func(subprueba *testing.T) {
			servidores := map[string]*httptest.Server{}
			for dominio := range rutaPorDominio {
				servidores[dominio] = servidorEco(subprueba, dominio)
			}
			pasarela := pasarelaPara(subprueba, servidores["scraping"].URL, map[string]string{
				"IDENTIDAD_URL": servidores["identidad"].URL, "CATALOGO_URL": servidores["catalogo"].URL,
				"RUTAS_URL": servidores["rutas"].URL, "IA_URL": servidores["ia"].URL,
			}, nil)
			servidores[caido].Close()
			casos := []casoEstado{{ruta: "/_gateway/ready", estado: http.StatusOK}}
			for dominio, ruta := range rutaPorDominio {
				if dominio == caido {
					casos = append(casos, casoEstado{ruta, http.StatusBadGateway, dominio + "_unavailable"})
				} else {
					casos = append(casos, casoEstado{ruta: ruta, estado: http.StatusOK})
				}
			}
			comprobarEstados(subprueba, pasarela, casos)
		})
	}
}

// TestTiempoLimitePorDominioSinRespaldo verifica el 504 propio de cada
// dominio y que la petición no se reintenta en Control.
func TestTiempoLimitePorDominioSinRespaldo(prueba *testing.T) {
	for _, caso := range []struct{ dominio, variable, ruta string }{
		{"catalogo", "CATALOGO_URL", "/api/v1/productos"},
		{"ia", "IA_URL", "/api/v1/chat"},
	} {
		prueba.Run(caso.dominio, func(subprueba *testing.T) {
			var llamadasControl atomic.Int32
			control := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) { llamadasControl.Add(1) }))
			defer control.Close()
			// Leer el cuerpo permite que net/http detecte el cierre de la conexión.
			lento := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
				_, _ = io.Copy(io.Discard, peticion.Body)
				<-peticion.Context().Done()
			}))
			defer lento.Close()
			pasarela := pasarelaPara(subprueba, control.URL, map[string]string{caso.variable: lento.URL, "GATEWAY_REQUEST_TIMEOUT": "40ms"}, nil)
			inicio := time.Now()
			respuesta, errorPeticion := http.Post(pasarela.URL+caso.ruta, "application/json", strings.NewReader(`{}`))
			if errorPeticion != nil {
				subprueba.Fatal(errorPeticion)
			}
			cuerpo := cuerpoDe(subprueba, respuesta)
			if respuesta.StatusCode != http.StatusGatewayTimeout || !strings.Contains(string(cuerpo), caso.dominio+"_timeout") ||
				llamadasControl.Load() != 0 || time.Since(inicio) > time.Second {
				subprueba.Fatalf("tiempo límite o respaldo incorrecto: %d %s", respuesta.StatusCode, cuerpo)
			}
		})
	}
}

// ===== Prefijos configurables =====

// TestPrefijosPropiosEnrutanRutasHijas reproduce SUP-350: con
// GATEWAY_CATALOGO_PREFIXES=/api/v1/productos/ las rutas hijas siguen
// llegando a Catálogo porque el "/" final se recorta al cargar, y un prefijo
// propio no captura rutas vecinas.
func TestPrefijosPropiosEnrutanRutasHijas(prueba *testing.T) {
	servidorRuta := func(nombre string) *httptest.Server {
		servidor := httptest.NewServer(http.HandlerFunc(func(respuesta http.ResponseWriter, peticion *http.Request) {
			respuesta.Header().Set(cabeceraUpstream, nombre)
			_, _ = io.WriteString(respuesta, peticion.URL.EscapedPath())
		}))
		prueba.Cleanup(servidor.Close)
		return servidor
	}
	control, catalogo := servidorRuta("scraping"), servidorRuta("catalogo")

	for _, escenario := range []struct {
		nombre, prefijos string
		rutas            map[string]string
	}{
		{
			nombre:   "barra_final",
			prefijos: "/api/v1/productos/",
			rutas: map[string]string{
				"/api/v1/productos": "catalogo", "/api/v1/productos/": "catalogo",
				"/api/v1/productos/123": "catalogo", "/api/v1/productos/a%2Fb": "catalogo",
				"/api/v1/productos-falso": "", "/api/v1/admin/productos": "",
				"/api/v1/scraper/trabajos": "scraping",
			},
		},
		{
			nombre:   "prefijos_propios",
			prefijos: "/api/v2/catalogo, /api/v1/ofertas/",
			rutas: map[string]string{
				"/api/v2/catalogo": "catalogo", "/api/v2/catalogo/item/9": "catalogo",
				"/api/v1/ofertas/semana": "catalogo", "/api/v2/catalogo-viejo": "",
				"/api/v1/productos": "", "/api/v1/productos/123": "",
				"/api/v1/scraper/trabajos": "scraping",
			},
		},
	} {
		prueba.Run(escenario.nombre, func(subprueba *testing.T) {
			pasarela := pasarelaPara(subprueba, control.URL, map[string]string{
				"CATALOGO_URL":              catalogo.URL,
				"GATEWAY_CATALOGO_PREFIXES": escenario.prefijos,
			}, nil)
			for ruta, destino := range escenario.rutas {
				respuesta, errorPeticion := http.Get(pasarela.URL + ruta)
				if errorPeticion != nil {
					subprueba.Fatal(errorPeticion)
				}
				cuerpo := cuerpoDe(subprueba, respuesta)
				if destino == "" {
					if respuesta.StatusCode != http.StatusNotFound {
						subprueba.Fatalf("%s: se esperaba 404, estado=%d", ruta, respuesta.StatusCode)
					}
					continue
				}
				if respuesta.StatusCode != http.StatusOK || respuesta.Header.Get(cabeceraUpstream) != destino || string(cuerpo) != ruta {
					subprueba.Fatalf("%s: estado=%d destino=%q ruta=%q, se esperaba %q", ruta, respuesta.StatusCode, respuesta.Header.Get(cabeceraUpstream), cuerpo, destino)
				}
			}
		})
	}
}

// TestEscrituraConservaIdempotencyKey verifica que la clave llega a Control.
func TestEscrituraConservaIdempotencyKey(prueba *testing.T) {
	control := servidorEco(prueba, "scraping")
	pasarela := pasarelaPara(prueba, control.URL, nil, nil)
	peticion, _ := http.NewRequest("POST", pasarela.URL+"/api/v1/scraper/productos", strings.NewReader(`{"fixture":true}`))
	peticion.Header.Set("Idempotency-Key", "lote-estable")
	respuesta, errorPeticion := http.DefaultClient.Do(peticion)
	if errorPeticion != nil {
		prueba.Fatal(errorPeticion)
	}
	if cuerpo := cuerpoDe(prueba, respuesta); respuesta.StatusCode != http.StatusOK || !strings.Contains(string(cuerpo), "lote-estable") {
		prueba.Fatalf("se perdió la clave de idempotencia: %d %s", respuesta.StatusCode, cuerpo)
	}
}
