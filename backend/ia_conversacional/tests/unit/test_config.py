"""
Tests unitarios para la validación central de configuración (app.config).

Verifica:
1. Fail-fast ante JWT_SECRET faltante o de longitud menor a 32 caracteres.
2. Fail-fast ante CATALOGO_INTERNAL_URL inválido (esquema no http/https, sin host, con credenciales, con path, con query o fragmento).
3. Fail-fast ante orígenes CORS inválidos (con comodín '*' o barra final '/').
4. Fail-fast ante timeouts y límites numéricos <= 0.
5. Aceptación explícita de CATALOGO_CACHE_TTL == 0.
6. Ausencia de llaves de API (GEMINI_API_KEY / GROQ_API_KEY) emite advertencia pero no lanza excepción en validar_configuracion.
"""

import pytest

from app import config


@pytest.fixture(autouse=True)
def configuracion_valida_base(monkeypatch):
    """Establece un entorno base 100% válido para aislar cada prueba unitaria."""
    monkeypatch.setattr(config, "JWT_SECRET", "secreto-seguro-para-pruebas-unitarias-con-mas-de-32-chars!")
    monkeypatch.setattr(config, "CATALOGO_INTERNAL_URL", "http://catalogo:8080")
    monkeypatch.setattr(config, "CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173")
    monkeypatch.setattr(config, "CATALOGO_TIMEOUT", 5.0)
    monkeypatch.setattr(config, "CATALOGO_PAGE_LIMIT", 100)
    monkeypatch.setattr(config, "CATALOGO_MAX_PAGINAS", 10)
    monkeypatch.setattr(config, "CATALOGO_CACHE_TTL", 60.0)
    monkeypatch.setattr(config, "GROQ_TIMEOUT", 15.0)
    monkeypatch.setattr(config, "GROQ_MAX_TOKENS_EXTRACCION", 512)
    monkeypatch.setattr(config, "GROQ_MAX_TOKENS_EXPLICACION", 200)
    monkeypatch.setattr(config, "GEMINI_TIMEOUT", 15.0)
    monkeypatch.setattr(config, "GEMINI_MAX_TOKENS_EXTRACCION", 1200)
    monkeypatch.setattr(config, "GEMINI_MAX_TOKENS_EXPLICACION", 2048)
    monkeypatch.setattr(config, "FALLBACK_BACKOFF_BASE", 1.5)
    monkeypatch.setattr(config, "FALLBACK_RETRIES_PRIMARIO", 4)
    monkeypatch.setattr(config, "FALLBACK_RETRIES_SECUNDARIO", 1)
    monkeypatch.setattr(config, "FALLBACK_JITTER_MAX", 0.5)
    monkeypatch.setattr(config, "CHAT_MAX_MENSAJE_LEN", 1000)
    monkeypatch.setattr(config, "EXPLICACION_MAX_PRODUCTOS", 8)
    monkeypatch.setattr(config, "GEMINI_API_KEY", "llave-gemini-prueba")
    monkeypatch.setattr(config, "GROQ_API_KEY", "llave-groq-prueba")


def test_validar_configuracion_exitosa_con_valores_validos():
    """Valida que una configuración íntegra no arroja excepciones."""
    config.validar_configuracion()


def test_validar_configuracion_falla_si_jwt_secret_falta(monkeypatch):
    """Verifica que la ausencia de JWT_SECRET lance RuntimeError."""
    monkeypatch.setattr(config, "JWT_SECRET", None)
    with pytest.raises(RuntimeError, match="JWT_SECRET debe estar configurado"):
        config.validar_configuracion()


def test_validar_configuracion_falla_si_jwt_secret_es_corto(monkeypatch):
    """Verifica que JWT_SECRET con menos de 32 caracteres lance RuntimeError."""
    monkeypatch.setattr(config, "JWT_SECRET", "clave_insegura_menor_32")
    with pytest.raises(RuntimeError, match="al menos 32 caracteres"):
        config.validar_configuracion()


def test_validar_configuracion_catalogo_url_invalido_esquema(monkeypatch):
    """Verifica que CATALOGO_INTERNAL_URL con esquema distinto a http o https falle."""
    monkeypatch.setattr(config, "CATALOGO_INTERNAL_URL", "ftp://catalogo:8080")
    with pytest.raises(RuntimeError, match="esquema no soportado"):
        config.validar_configuracion()


def test_validar_configuracion_catalogo_url_invalido_sin_host(monkeypatch):
    """Verifica que CATALOGO_INTERNAL_URL sin netloc/host falle."""
    monkeypatch.setattr(config, "CATALOGO_INTERNAL_URL", "http://")
    with pytest.raises(RuntimeError, match="debe incluir un host"):
        config.validar_configuracion()


def test_validar_configuracion_catalogo_url_invalido_con_credenciales(monkeypatch):
    """Verifica que CATALOGO_INTERNAL_URL no permita credenciales en la URL."""
    monkeypatch.setattr(config, "CATALOGO_INTERNAL_URL", "http://usuario:secreto@catalogo:8080")
    with pytest.raises(RuntimeError, match="no debe incluir credenciales"):
        config.validar_configuracion()


def test_validar_configuracion_catalogo_url_invalido_con_path(monkeypatch):
    """Verifica que CATALOGO_INTERNAL_URL no contenga path (como /api)."""
    monkeypatch.setattr(config, "CATALOGO_INTERNAL_URL", "http://catalogo:8080/api")
    with pytest.raises(RuntimeError, match="no debe incluir path"):
        config.validar_configuracion()


def test_validar_configuracion_catalogo_url_invalido_con_query_o_fragmento(monkeypatch):
    """Verifica que CATALOGO_INTERNAL_URL no contenga query parameters ni fragmentos."""
    monkeypatch.setattr(config, "CATALOGO_INTERNAL_URL", "http://catalogo:8080?filtro=activo")
    with pytest.raises(RuntimeError, match="query parameters ni fragmentos"):
        config.validar_configuracion()

    monkeypatch.setattr(config, "CATALOGO_INTERNAL_URL", "http://catalogo:8080#seccion")
    with pytest.raises(RuntimeError, match="query parameters ni fragmentos"):
        config.validar_configuracion()


def test_validar_configuracion_cors_invalido_con_comodin(monkeypatch):
    """Verifica que orígenes CORS con '*' sean rechazados."""
    monkeypatch.setattr(config, "CORS_ALLOWED_ORIGINS", "http://localhost:3000,*")
    with pytest.raises(RuntimeError, match="no puede contener comodines"):
        config.validar_configuracion()


def test_validar_configuracion_cors_invalido_con_barra_final(monkeypatch):
    """Verifica que orígenes CORS terminados en '/' sean rechazados."""
    monkeypatch.setattr(config, "CORS_ALLOWED_ORIGINS", "http://localhost:3000/")
    with pytest.raises(RuntimeError, match="no puede terminar en barra"):
        config.validar_configuracion()


def test_validar_configuracion_timeouts_y_limites_menores_o_iguales_a_cero(monkeypatch):
    """Verifica que timeouts y límites numéricos <= 0 lancen RuntimeError."""
    parametros_estrictos = [
        ("CATALOGO_TIMEOUT", 0),
        ("CATALOGO_TIMEOUT", -1.0),
        ("CATALOGO_PAGE_LIMIT", 0),
        ("CATALOGO_MAX_PAGINAS", 0),
        ("GROQ_TIMEOUT", 0),
        ("GROQ_MAX_TOKENS_EXTRACCION", 0),
        ("GROQ_MAX_TOKENS_EXPLICACION", 0),
        ("GEMINI_TIMEOUT", 0),
        ("GEMINI_MAX_TOKENS_EXTRACCION", 0),
        ("GEMINI_MAX_TOKENS_EXPLICACION", 0),
        ("FALLBACK_BACKOFF_BASE", 0),
        ("CHAT_MAX_MENSAJE_LEN", 0),
        ("EXPLICACION_MAX_PRODUCTOS", 0),
    ]

    for nombre_parametro, valor_invalido in parametros_estrictos:
        monkeypatch.setattr(config, nombre_parametro, valor_invalido)
        with pytest.raises(RuntimeError, match=f"{nombre_parametro} debe ser mayor a 0"):
            config.validar_configuracion()
        # Restaurar a valor válido antes del siguiente chequeo
        monkeypatch.setattr(config, nombre_parametro, 10.0 if "TIMEOUT" in nombre_parametro or "BASE" in nombre_parametro else 10)


def test_validar_configuracion_acepta_catalogo_cache_ttl_cero(monkeypatch):
    """Verifica que CATALOGO_CACHE_TTL == 0 sea admitido, pero < 0 sea rechazado."""
    monkeypatch.setattr(config, "CATALOGO_CACHE_TTL", 0.0)
    config.validar_configuracion()

    monkeypatch.setattr(config, "CATALOGO_CACHE_TTL", -0.5)
    with pytest.raises(RuntimeError, match="CATALOGO_CACHE_TTL debe ser mayor o igual a 0"):
        config.validar_configuracion()


def test_validar_configuracion_sin_api_keys_no_lanza_excepcion(monkeypatch):
    """Verifica que la ausencia de GEMINI_API_KEY y GROQ_API_KEY no impida arrancar la app."""
    monkeypatch.setattr(config, "GEMINI_API_KEY", None)
    monkeypatch.setattr(config, "GROQ_API_KEY", None)
    config.validar_configuracion()


def test_validar_configuracion_catalogo_url_acepta_barra_final(monkeypatch):
    """Verifica que una barra final unica en CATALOGO_INTERNAL_URL sea aceptada sin error (H-04)."""
    monkeypatch.setattr(config, "CATALOGO_INTERNAL_URL", "http://catalogo:8080/")
    config.validar_configuracion()


def test_validar_configuracion_falla_si_cors_esta_vacio(monkeypatch):
    """Verifica fail-fast si CORS_ALLOWED_ORIGINS queda vacio (H-08)."""
    monkeypatch.setattr(config, "CORS_ALLOWED_ORIGINS", "   ,  ")
    with pytest.raises(RuntimeError, match="al menos un origen valido"):
        config.validar_configuracion()


def test_lectores_env_toleran_cadena_vacia_y_fallan_con_texto_invalido(monkeypatch):
    """Verifica que _leer_env_float y _leer_env_int usen default en vacio y lancen RuntimeError en basura (H-08)."""
    monkeypatch.setenv("VAR_PRUEBA_FLOAT", "")
    assert config._leer_env_float("VAR_PRUEBA_FLOAT", 5.0) == 5.0

    monkeypatch.setenv("VAR_PRUEBA_INT", "   ")
    assert config._leer_env_int("VAR_PRUEBA_INT", 10) == 10

    monkeypatch.setenv("VAR_PRUEBA_FLOAT", "no-numero")
    with pytest.raises(RuntimeError, match="decimal valido"):
        config._leer_env_float("VAR_PRUEBA_FLOAT", 5.0)

    monkeypatch.setenv("VAR_PRUEBA_INT", "abc")
    with pytest.raises(RuntimeError, match="entero valido"):
        config._leer_env_int("VAR_PRUEBA_INT", 10)


def test_lector_env_float_rechaza_valores_no_finitos(monkeypatch):
    """Verifica que _leer_env_float rechace nan, inf y -inf con RuntimeError."""
    for valor_no_finito in ("nan", "NaN", "inf", "-inf", "+inf", "Infinity", "-Infinity"):
        monkeypatch.setenv("VAR_PRUEBA_FLOAT", valor_no_finito)
        with pytest.raises(RuntimeError, match="decimal valido y finito"):
            config._leer_env_float("VAR_PRUEBA_FLOAT", 5.0)


def test_validar_configuracion_catalogo_url_invalido_puerto_y_host(monkeypatch):
    """Verifica que CATALOGO_INTERNAL_URL rechace hosts vacios y puertos fuera de rango."""
    casos_invalidos = [
        "http://:8080",
        "http://catalogo:puerto",
        "http://catalogo:99999",
        "http://catalogo:0",
        "http://catalogo:-80",
    ]
    for url_invalida in casos_invalidos:
        monkeypatch.setattr(config, "CATALOGO_INTERNAL_URL", url_invalida)
        with pytest.raises(RuntimeError):
            config.validar_configuracion()


def test_validar_configuracion_cors_invalido_formato_estricto(monkeypatch):
    """Verifica que CORS rechace origenes sin scheme, con credenciales, rutas, query o fragmento."""
    origenes_invalidos = [
        "localhost:3000",
        "http://localhost:3000/ruta",
        "https://usuario:clave@ejemplo.cl",
        "https://ejemplo.cl?parametro=valor",
        "https://ejemplo.cl#seccion",
        "http://ejemplo.cl:99999",
    ]
    for origen_invalido in origenes_invalidos:
        monkeypatch.setattr(config, "CORS_ALLOWED_ORIGINS", origen_invalido)
        with pytest.raises(RuntimeError):
            config.validar_configuracion()


def test_validar_configuracion_advertencia_gemini_endpoint_vs_modelo(monkeypatch, caplog):
    """Verifica que se emita advertencia si GEMINI_ENDPOINT y GEMINI_MODEL divergen."""
    monkeypatch.setenv("GEMINI_ENDPOINT", "https://generativelanguage.googleapis.com/v1beta/models/otro-modelo:generateContent")
    monkeypatch.setenv("GEMINI_MODEL", "gemini-3.1-flash-lite")
    with caplog.at_level("WARNING"):
        config.validar_configuracion()
    assert "esta configurado explicitamente y anula a GEMINI_MODEL" in caplog.text
