// Package scraping es el cliente HTTP del bot hacia Control de scraping (a
// través del Gateway): crea trabajos, encola su ejecución y consulta su estado.
// Se autentica con el token de servicio compartido (SCRAPING_SERVICE_TOKEN),
// que nunca sale de este paquete ni se expone a Discord.
package scraping

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ===== Constantes del contrato con Control =====

const (
	// EstadoEncolado, EstadoEnProgreso, EstadoCompletado y EstadoFallido son los
	// valores del campo "estado" que Control devuelve para un trabajo.
	EstadoEncolado   = "encolado"
	EstadoEnProgreso = "en_progreso"
	EstadoCompletado = "completado"
	EstadoFallido    = "fallido"

	cabeceraAutorizacion  = "Authorization"
	prefijoBearer         = "Bearer "
	cabeceraTipoContenido = "Content-Type"
	tipoContenidoJSON     = "application/json"
	sufijoEjecutar        = "/ejecutar"
)

// ===== Cliente =====

// ErrComunicacion marca los fallos de red hacia Control (conexión, timeout o
// cancelación), para que quien llama los distinga de una respuesta HTTP
// recibida pero inválida.
var ErrComunicacion = errors.New("falló la comunicación con Control")

// Cliente habla con Control en nombre del bot.
type Cliente struct {
	urlBase     string
	token       string
	clienteHTTP *http.Client
}

// EstadoTrabajo contiene los campos de un trabajo que el bot muestra a sus usuarios.
type EstadoTrabajo struct {
	Estado             string
	Cadena             string
	ElementosExtraidos int
	RegistroErrores    string
}

// NuevoCliente construye el cliente. Falla si falta el token de servicio o la
// URL: sin ellos todas las peticiones serían rechazadas por Control, y es un
// error de configuración que no debe descubrirse recién al primer comando.
func NuevoCliente(urlBase, tokenServicio string, timeout time.Duration) (*Cliente, error) {
	if strings.TrimSpace(tokenServicio) == "" {
		return nil, errors.New("el token de servicio de scraping es obligatorio")
	}
	if strings.TrimSpace(urlBase) == "" {
		return nil, errors.New("la URL base de Control es obligatoria")
	}
	return &Cliente{
		urlBase:     strings.TrimSuffix(urlBase, "/"),
		token:       tokenServicio,
		clienteHTTP: &http.Client{Timeout: timeout},
	}, nil
}

// ===== Operaciones sobre trabajos =====

// CrearTrabajo registra un trabajo para la cadena y devuelve su identificador.
func (cliente *Cliente) CrearTrabajo(contexto context.Context, cadenaID int, usuarioID string) (string, error) {
	cuerpoJSON, errorObtenido := json.Marshal(map[string]interface{}{
		"cadena_id": cadenaID, "disparado_por_usuario_id": usuarioID,
	})
	if errorObtenido != nil {
		return "", fmt.Errorf("no se pudo serializar la creación: %w", errorObtenido)
	}
	respuesta, errorObtenido := cliente.enviar(contexto, http.MethodPost, cliente.urlBase, cuerpoJSON)
	if errorObtenido != nil {
		return "", fmt.Errorf("%w: %w", ErrComunicacion, errorObtenido)
	}
	defer respuesta.Body.Close()
	if respuesta.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("Control respondió HTTP %d al crear", respuesta.StatusCode)
	}
	trabajoID, esValido := leerIDTrabajo(respuesta)
	if !esValido {
		return "", errors.New("Control devolvió un identificador de trabajo inválido")
	}
	return trabajoID, nil
}

// EjecutarTrabajo encola el rastreador indicado en un trabajo ya creado. Que
// Control acepte la solicitud (202) no significa que la extracción haya
// terminado: el resultado se conoce solo consultando el estado.
func (cliente *Cliente) EjecutarTrabajo(contexto context.Context, trabajoID, nombreRastreador string) error {
	cuerpoJSON, errorObtenido := json.Marshal(map[string]string{"spider": nombreRastreador})
	if errorObtenido != nil {
		return fmt.Errorf("no se pudo serializar la ejecución: %w", errorObtenido)
	}
	respuesta, errorObtenido := cliente.enviar(contexto, http.MethodPost, cliente.urlBase+"/"+trabajoID+sufijoEjecutar, cuerpoJSON)
	if errorObtenido != nil {
		return fmt.Errorf("%w: %w", ErrComunicacion, errorObtenido)
	}
	defer respuesta.Body.Close()
	if respuesta.StatusCode != http.StatusAccepted {
		return fmt.Errorf("Control respondió HTTP %d al ejecutar", respuesta.StatusCode)
	}
	return nil
}

// ConsultarEstado obtiene el estado actual de un trabajo y valida que la
// respuesta tenga la forma esperada antes de mostrarla a un usuario.
func (cliente *Cliente) ConsultarEstado(contexto context.Context, trabajoID string) (EstadoTrabajo, error) {
	respuesta, errorObtenido := cliente.enviar(contexto, http.MethodGet, cliente.urlBase+"/"+trabajoID, nil)
	if errorObtenido != nil {
		return EstadoTrabajo{}, fmt.Errorf("%w: %w", ErrComunicacion, errorObtenido)
	}
	defer respuesta.Body.Close()
	return leerEstadoTrabajo(respuesta)
}

// ===== Transporte y lectura de respuestas =====

// enviar ejecuta una petición autenticada con el token de servicio. El cuerpo,
// si existe, siempre es JSON.
func (cliente *Cliente) enviar(contexto context.Context, metodo, direccion string, cuerpoJSON []byte) (*http.Response, error) {
	var cuerpo io.Reader
	if cuerpoJSON != nil {
		cuerpo = bytes.NewReader(cuerpoJSON)
	}
	peticion, errorObtenido := http.NewRequestWithContext(contexto, metodo, direccion, cuerpo)
	if errorObtenido != nil {
		return nil, errorObtenido
	}
	if cuerpoJSON != nil {
		peticion.Header.Set(cabeceraTipoContenido, tipoContenidoJSON)
	}
	peticion.Header.Set(cabeceraAutorizacion, prefijoBearer+cliente.token)
	return cliente.clienteHTTP.Do(peticion)
}

// leerEstadoTrabajo valida una respuesta de estado: rechaza códigos distintos
// de 200 y cuerpos sin "estado" o "cadena_nombre" en lugar de entrar en pánico
// o mostrar datos vacíos al usuario.
func leerEstadoTrabajo(respuesta *http.Response) (EstadoTrabajo, error) {
	if respuesta.StatusCode != http.StatusOK {
		return EstadoTrabajo{}, fmt.Errorf("HTTP %d", respuesta.StatusCode)
	}
	var cuerpo map[string]interface{}
	if errorObtenido := json.NewDecoder(respuesta.Body).Decode(&cuerpo); errorObtenido != nil {
		return EstadoTrabajo{}, fmt.Errorf("respuesta inválida: %w", errorObtenido)
	}
	estado, estadoValido := cuerpo["estado"].(string)
	cadena, cadenaValida := cuerpo["cadena_nombre"].(string)
	if !estadoValido || !cadenaValida {
		return EstadoTrabajo{}, errors.New("respuesta incompleta")
	}
	resultado := EstadoTrabajo{Estado: estado, Cadena: cadena}
	if elementos, existe := cuerpo["elementos_extraidos"].(float64); existe {
		resultado.ElementosExtraidos = int(elementos)
	}
	if mensajeErrores, existe := cuerpo["registro_errores"].(string); existe {
		resultado.RegistroErrores = mensajeErrores
	}
	return resultado, nil
}

// leerIDTrabajo extrae el identificador de un trabajo recién creado.
func leerIDTrabajo(respuesta *http.Response) (string, bool) {
	var cuerpo map[string]interface{}
	if errorObtenido := json.NewDecoder(respuesta.Body).Decode(&cuerpo); errorObtenido != nil {
		return "", false
	}
	identificador, existe := cuerpo["id"].(string)
	return identificador, existe && identificador != ""
}
