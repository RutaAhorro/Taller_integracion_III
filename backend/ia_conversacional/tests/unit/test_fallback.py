"""
Tests unitarios para el proveedor de fallback y resiliencia (FallbackProveedor).

Verifica:
1. Clasificación estricta de _es_error_recuperable (transitorios 429, 500, 502, 503, 504,
   TimeoutException y RequestError retornan True; errores cliente 400 y 401 retornan False).
2. FallbackProveedor.generar_explicacion con proveedores simulados en memoria:
   - Reintenta proveedor primario ante error recuperable con backoff y conmuta a secundario.
   - Conmuta directamente sin reintentar ante error 400 no recuperable.
"""

from unittest.mock import AsyncMock, MagicMock

import httpx
import pytest

from app.domain.proveedores import ProveedorIA, ResultadoExtraccion
from app.repositories.gemini import GeminiProveedor
from app.repositories.groq import GroqProveedor
from app.services.fallback import FallbackProveedor, formatear_error_seguro


class ProveedorFalso(ProveedorIA):
    """Proveedor auxiliar en memoria para pruebas deterministas."""

    def __init__(self, nombre_proveedor: str):
        self.nombre = nombre_proveedor
        self.llamadas_explicacion = 0
        self.llamadas_extraccion = 0

    async def extraer_filtros(self, mensaje_usuario: str):
        self.llamadas_extraccion += 1
        return ResultadoExtraccion(filtros={}, tiempo_extraccion_ms=1)

    async def generar_explicacion(self, mensaje_usuario: str, productos_filtrados: list) -> str:
        self.llamadas_explicacion += 1
        return f"Explicacion generada por {self.nombre}"


def test_es_error_recuperable_clasificacion():
    """Verifica que solo errores de red y estados transitorios sean marcados como recuperables."""
    proveedor_fallback = FallbackProveedor(proveedores=[])

    # Errores de red y timeouts
    assert proveedor_fallback._es_error_recuperable(httpx.TimeoutException("Tiempo agotado")) is True
    assert proveedor_fallback._es_error_recuperable(httpx.RequestError("Fallo de socket")) is True

    # Códigos HTTP recuperables (rate limit y errores transitorios de servidor)
    for codigo_estado in [429, 500, 502, 503, 504]:
        mock_respuesta = MagicMock(spec=httpx.Response)
        mock_respuesta.status_code = codigo_estado
        error_http = httpx.HTTPStatusError("Error transitorio", request=MagicMock(), response=mock_respuesta)
        assert proveedor_fallback._es_error_recuperable(error_http) is True

    # Códigos HTTP no recuperables (errores de cliente definitivos)
    for codigo_estado in [400, 401, 403, 404, 422]:
        mock_respuesta = MagicMock(spec=httpx.Response)
        mock_respuesta.status_code = codigo_estado
        error_http = httpx.HTTPStatusError("Error cliente", request=MagicMock(), response=mock_respuesta)
        assert proveedor_fallback._es_error_recuperable(error_http) is False

    # Errores genéricos
    assert proveedor_fallback._es_error_recuperable(ValueError("Parametro invalido")) is False
    assert proveedor_fallback._es_error_recuperable(RuntimeError("Fallo interno")) is False


@pytest.mark.anyio
async def test_generar_explicacion_reintenta_primario_recuperable_y_conmuta_a_secundario(monkeypatch):
    """Verifica que un error recuperable reintente según max_retries y luego conmute al secundario."""
    mock_dormir = AsyncMock()
    monkeypatch.setattr("app.services.fallback.asyncio.sleep", mock_dormir)

    proveedor_primario = ProveedorFalso("primario")
    proveedor_secundario = ProveedorFalso("secundario")

    # Primario falla con timeout siempre
    async def primario_falla(*args, **kwargs):
        proveedor_primario.llamadas_explicacion += 1
        raise httpx.TimeoutException("Timeout simulado en primario")

    proveedor_primario.generar_explicacion = primario_falla

    orquestador = FallbackProveedor(
        proveedores=[proveedor_primario, proveedor_secundario],
        reintentos_primario=2,
        reintentos_secundario=1,
        espera_base=0.01,
        variacion_maxima=0.0,
    )

    resultado = await orquestador.generar_explicacion(
        mensaje_usuario="quiero verduras",
        productos_filtrados=[],
    )

    assert resultado == "Explicacion generada por secundario"
    # 1 intento inicial + 2 reintentos = 3 llamadas al primario
    assert proveedor_primario.llamadas_explicacion == 3
    # 1 llamada exitosa al secundario
    assert proveedor_secundario.llamadas_explicacion == 1
    # Se invocó asyncio.sleep 2 veces para los reintentos del primario
    assert mock_dormir.await_count == 2


@pytest.mark.anyio
async def test_generar_explicacion_no_reintenta_error_400_no_recuperable(monkeypatch):
    """Verifica que un error HTTP 400 en primario no realice reintentos y conmute inmediatamente al secundario."""
    mock_dormir = AsyncMock()
    monkeypatch.setattr("app.services.fallback.asyncio.sleep", mock_dormir)

    proveedor_primario = ProveedorFalso("primario")
    proveedor_secundario = ProveedorFalso("secundario")

    mock_respuesta = MagicMock(spec=httpx.Response)
    mock_respuesta.status_code = 400
    error_400 = httpx.HTTPStatusError("Bad Request", request=MagicMock(), response=mock_respuesta)

    async def primario_error_400(*args, **kwargs):
        proveedor_primario.llamadas_explicacion += 1
        raise error_400

    proveedor_primario.generar_explicacion = primario_error_400

    orquestador = FallbackProveedor(
        proveedores=[proveedor_primario, proveedor_secundario],
        reintentos_primario=3,
        reintentos_secundario=1,
    )

    resultado = await orquestador.generar_explicacion(
        mensaje_usuario="quiero pan",
        productos_filtrados=[],
    )

    assert resultado == "Explicacion generada por secundario"
    # Exactamente 1 llamada en primario sin reintentos
    assert proveedor_primario.llamadas_explicacion == 1
    assert proveedor_secundario.llamadas_explicacion == 1
    # Cero llamadas a sleep porque no hubo reintentos
    assert mock_dormir.await_count == 0


@pytest.mark.anyio
async def test_fallback_extraer_filtros_conmuta_gemini_429_a_groq(monkeypatch):
    """Verifica que FallbackProveedor extrae filtros: ante un 429 recuperable en Gemini reintenta y conmuta a Groq."""
    mock_dormir = AsyncMock()
    monkeypatch.setattr("app.services.fallback.asyncio.sleep", mock_dormir)

    proveedor_gemini = ProveedorFalso("gemini")

    mock_respuesta = MagicMock(spec=httpx.Response)
    mock_respuesta.status_code = 429
    error_429 = httpx.HTTPStatusError("Rate limit", request=MagicMock(), response=mock_respuesta)

    async def gemini_falla(*args, **kwargs):
        proveedor_gemini.llamadas_extraccion += 1
        raise error_429

    proveedor_gemini.extraer_filtros = gemini_falla

    proveedor_groq = ProveedorFalso("groq")

    orquestador = FallbackProveedor(
        proveedores=[proveedor_gemini, proveedor_groq],
        reintentos_primario=1,
        reintentos_secundario=0,
        espera_base=0.01,
        variacion_maxima=0.0,
    )

    resultado = await orquestador.extraer_filtros(mensaje="quiero verduras")

    assert resultado.error is None
    assert resultado.filtros == {}
    # El 429 fue recuperable: un reintento en Gemini y luego la conmutacion a Groq.
    assert proveedor_gemini.llamadas_extraccion == 2
    assert proveedor_groq.llamadas_extraccion == 1
    assert mock_dormir.await_count == 1


@pytest.mark.anyio
async def test_fallback_extraer_filtros_sin_clave_api_conmuta_sin_reintentos(monkeypatch):
    """Verifica que un proveedor sin clave API conmute al siguiente sin reintentos ni sleep."""
    mock_dormir = AsyncMock()
    monkeypatch.setattr("app.services.fallback.asyncio.sleep", mock_dormir)

    proveedor_gemini_sin_clave = GeminiProveedor(clave_api=None, url_endpoint="https://ejemplo.com/gemini")
    proveedor_groq = ProveedorFalso("groq")

    orquestador = FallbackProveedor(
        proveedores=[proveedor_gemini_sin_clave, proveedor_groq],
        reintentos_primario=3,
        reintentos_secundario=3,
        espera_base=5.0,
        variacion_maxima=0.0,
    )

    resultado = await orquestador.extraer_filtros(mensaje="quiero verduras")

    assert resultado.error is None
    assert resultado.filtros == {}
    # Error no recuperable (clave ausente): conmuta sin reintentos ni espera.
    assert mock_dormir.await_count == 0
    assert proveedor_groq.llamadas_extraccion == 1


@pytest.mark.anyio
async def test_fallback_log_incluye_tipo_excepcion_y_oculta_key_en_url(caplog):
    """Verifica que el log de fallo incluya el tipo de excepción aunque el mensaje esté vacío y oculte key=."""
    proveedor_timeout = ProveedorFalso("gemini")

    async def falla_timeout_vacio(*args, **kwargs):
        raise httpx.ReadTimeout("")

    proveedor_timeout.generar_explicacion = falla_timeout_vacio

    proveedor_url_secreta = ProveedorFalso("groq")

    async def falla_con_key(*args, **kwargs):
        raise httpx.RequestError(
            "Fallo llamando a https://generativelanguage.googleapis.com/v1beta?key=secreto123&alt=json"
        )

    proveedor_url_secreta.generar_explicacion = falla_con_key

    orquestador = FallbackProveedor(
        proveedores=[proveedor_timeout, proveedor_url_secreta],
        reintentos_primario=0,
        reintentos_secundario=0,
    )

    with caplog.at_level("WARNING"):
        with pytest.raises(RuntimeError, match="key=\\*\\*\\*"):
            await orquestador.generar_explicacion(mensaje_usuario="hola", productos_filtrados=[])

    assert "Falló la explicación en gemini tras 1 intentos: ReadTimeout" in caplog.text
    assert "Falló la explicación en groq tras 1 intentos: RequestError:" in caplog.text
    assert "secreto123" not in caplog.text
    assert "key=***" in caplog.text


@pytest.mark.anyio
async def test_fallback_logs_distinguen_extraccion_de_explicacion_sin_claves_ni_cabeceras(caplog):
    """Los logs distinguen fallo en extracción frente a explicación y ocultan claves y cabeceras."""
    proveedor_falla_extraccion = ProveedorFalso("gemini")
    proveedor_falla_explicacion = ProveedorFalso("groq")

    async def extraccion_con_cabecera(*args, **kwargs):
        raise ValueError("Rechazado con x-goog-api-key: clave-google-secreta")

    async def explicacion_con_bearer(*args, **kwargs):
        raise RuntimeError("Error con Authorization: Bearer token-groq-secreto")

    proveedor_falla_extraccion.extraer_filtros = extraccion_con_cabecera
    proveedor_falla_explicacion.generar_explicacion = explicacion_con_bearer

    orquestador_extraccion = FallbackProveedor(
        proveedores=[proveedor_falla_extraccion],
        reintentos_primario=0,
    )
    with caplog.at_level("WARNING"):
        resultado = await orquestador_extraccion.extraer_filtros(mensaje="leche")

    assert resultado.error is not None
    assert "Falló la extracción" in caplog.text
    assert "Falló la explicación" not in caplog.text
    assert "clave-google-secreta" not in caplog.text
    assert "clave-google-secreta" not in resultado.error

    caplog.clear()

    orquestador_explicacion = FallbackProveedor(
        proveedores=[proveedor_falla_explicacion],
        reintentos_primario=0,
    )
    with caplog.at_level("WARNING"):
        with pytest.raises(RuntimeError) as error_capturado:
            await orquestador_explicacion.generar_explicacion(mensaje_usuario="leche", productos_filtrados=[])

    assert "Falló la explicación" in caplog.text
    assert "Falló la extracción" not in caplog.text
    assert "token-groq-secreto" not in caplog.text
    assert "token-groq-secreto" not in str(error_capturado.value)


def test_formatear_error_seguro_enmascara_cabeceras_ilegales_diccionarios_basic_y_x_api_key():
    """Oculta credenciales en LocalProtocolError, diccionarios serializados, Authorization Basic y x-api-key."""
    casos = [
        (
            httpx.LocalProtocolError("Illegal header value b'AQ.clave-falsa-123\\n'"),
            "AQ.clave-falsa-123",
            "Illegal header value b'***'",
        ),
        (
            ValueError("Cabeceras: {'x-goog-api-key': 'clave-falsa-comilla-simple'}"),
            "clave-falsa-comilla-simple",
            "{'x-goog-api-key': '***'}",
        ),
        (
            ValueError('Cabeceras: {"x-goog-api-key": "clave-falsa-comilla-doble"}'),
            "clave-falsa-comilla-doble",
            '{"x-goog-api-key": "***"}',
        ),
        (
            RuntimeError("Fallo con Authorization: Basic dXN1YXJpbzpjbGF2ZS1mYWxzYQ=="),
            "dXN1YXJpbzpjbGF2ZS1mYWxzYQ==",
            "Authorization: Basic ***",
        ),
        (
            RuntimeError("Fallo con x-api-key: clave-falsa-x-api-key"),
            "clave-falsa-x-api-key",
            "x-api-key: ***",
        ),
    ]

    for error_prueba, secreto_falso, fragmento_enmascarado in casos:
        formateado = formatear_error_seguro(error_prueba)
        assert secreto_falso not in formateado
        assert fragmento_enmascarado in formateado


@pytest.mark.anyio
async def test_groq_proveedor_no_loguea_cuerpo_crudo_ante_error_http(caplog):
    """Si Groq devuelve un error HTTP sin filtros rescatables, el log no expone el cuerpo de respuesta."""
    proveedor = GroqProveedor(
        clave_api="clave-falsa-groq",
        url_endpoint="https://api.groq.test/v1/chat/completions",
        modelo_extraccion="modelo-extraccion",
        modelo_explicacion="modelo-explicacion",
    )
    peticion = httpx.Request("POST", proveedor.url_endpoint)
    respuesta = httpx.Response(401, text="Unauthorized: Bearer clave-falsa-en-cuerpo", request=peticion)

    async def enviar_con_error(_cuerpo: dict) -> dict:
        raise httpx.HTTPStatusError("401 Unauthorized", request=peticion, response=respuesta)

    proveedor._enviar = enviar_con_error

    with caplog.at_level("WARNING"):
        with pytest.raises(httpx.HTTPStatusError):
            await proveedor.extraer_filtros("leche")

    assert "401" in caplog.text
    assert "clave-falsa-en-cuerpo" not in caplog.text


@pytest.mark.anyio
async def test_gemini_y_groq_envian_razonamiento_acotado_y_reportan_nombre_proveedor():
    """Gemini usa thinkingLevel='minimal' y Groq usa reasoning_effort='low'; ambos informan su nombre."""
    cuerpos_gemini: list[dict] = []
    proveedor_gemini = GeminiProveedor(clave_api="clave-gemini", url_endpoint="https://ejemplo.invalid/gemini")

    async def enviar_gemini(cuerpo: dict) -> dict:
        cuerpos_gemini.append(cuerpo)
        return {
            "candidates": [
                {
                    "content": {
                        "parts": [
                            {
                                "functionCall": {
                                    "name": "extraer_filtros_busqueda",
                                    "args": {"palabras_clave": ["leche"]},
                                }
                            }
                        ]
                    }
                }
            ]
        }

    proveedor_gemini._enviar = enviar_gemini
    resultado_gemini = await proveedor_gemini.extraer_filtros("Busco leche")

    assert resultado_gemini.proveedor == "gemini"
    assert cuerpos_gemini[0]["generationConfig"]["thinkingConfig"]["thinkingLevel"] == "minimal"
    assert cuerpos_gemini[0]["generationConfig"]["maxOutputTokens"] == 256

    cuerpos_groq: list[dict] = []
    proveedor_groq = GroqProveedor(
        clave_api="clave-groq",
        url_endpoint="https://ejemplo.invalid/groq",
        modelo_extraccion="openai/gpt-oss-20b",
        modelo_explicacion="openai/gpt-oss-120b",
    )

    async def enviar_groq(cuerpo: dict) -> dict:
        cuerpos_groq.append(cuerpo)
        return {
            "choices": [{"message": {"tool_calls": [{"function": {"arguments": '{"palabras_clave": ["leche"]}'}}]}}]
        }

    proveedor_groq._enviar = enviar_groq
    resultado_groq = await proveedor_groq.extraer_filtros("Busco leche")

    assert resultado_groq.proveedor == "groq"
    assert cuerpos_groq[0]["reasoning_effort"] == "low"
    assert cuerpos_groq[0]["max_tokens"] == 256


@pytest.mark.anyio
async def test_fallback_groq_primario_no_llama_a_gemini_cuando_groq_responde():
    """Con orden Groq -> Gemini, si Groq resuelve extracción y explicación no se invoca a Gemini."""
    proveedor_groq = ProveedorFalso("groq")
    proveedor_gemini = ProveedorFalso("gemini")

    orquestador = FallbackProveedor(
        proveedores=[proveedor_groq, proveedor_gemini],
        reintentos_primario=0,
        reintentos_secundario=0,
    )

    extraccion = await orquestador.extraer_filtros("Busco aceite de oliva")
    explicacion = await orquestador.generar_explicacion("Busco aceite de oliva", [])

    assert extraccion.proveedor == "groq"
    assert explicacion == "Explicacion generada por groq"
    assert proveedor_groq.llamadas_extraccion == 1
    assert proveedor_groq.llamadas_explicacion == 1
    assert proveedor_gemini.llamadas_extraccion == 0
    assert proveedor_gemini.llamadas_explicacion == 0


@pytest.mark.anyio
async def test_groq_proveedor_rescata_tool_use_failed_sin_conmutar_a_gemini(caplog):
    """Ante HTTP 400 `tool_use_failed` con JSON rescatable, Groq extrae filtros sin caer a Gemini ni loguear valores."""
    proveedor_groq = GroqProveedor(
        clave_api="clave-groq",
        url_endpoint="https://api.groq.test/v1/chat/completions",
        modelo_extraccion="openai/gpt-oss-20b",
        modelo_explicacion="openai/gpt-oss-120b",
    )
    peticion = httpx.Request("POST", proveedor_groq.url_endpoint)
    respuesta_400 = httpx.Response(
        400,
        json={
            "error": {
                "code": "tool_use_failed",
                "failed_generation": (
                    '<function=extraer_filtros_busqueda>{"palabras_clave": ["aceite de oliva"]}</function>'
                ),
            }
        },
        request=peticion,
    )

    async def enviar_tool_use_failed(_cuerpo: dict) -> dict:
        raise httpx.HTTPStatusError("400 Bad Request", request=peticion, response=respuesta_400)

    proveedor_groq._enviar = enviar_tool_use_failed
    proveedor_gemini = ProveedorFalso("gemini")

    orquestador = FallbackProveedor(
        proveedores=[proveedor_groq, proveedor_gemini],
        reintentos_primario=0,
        reintentos_secundario=0,
    )

    with caplog.at_level("INFO"):
        resultado = await orquestador.extraer_filtros("Busco aceite de oliva")

    assert resultado.error is None
    assert resultado.proveedor == "groq"
    assert resultado.filtros == {"palabras_clave": ["aceite de oliva"]}
    assert proveedor_gemini.llamadas_extraccion == 0
    assert "palabras_clave" in caplog.text
    assert "aceite de oliva" not in caplog.text
