# IA conversacional — contexto IA

- FastAPI, puerto 8002, entrada `app/main.py`. Existe en Compose; no figura
  como desplegado en el inventario ni tiene manifiestos K8s.
- Capas como Identidad: `app/api`, `app/services`, `app/repositories`,
  `app/domain` y `app/config.py`.
- `app/api/chat.py`: `POST /api/v1/chat` con JWT de Identidad (`app/api/sesion.py`).
  Gateway lo enruta con `IA_URL`/`GATEWAY_IA_PREFIXES`; el frontend llama
  `/api/v1/chat` por Gateway y solo lee `respuesta`.
- `app/repositories/catalogo.py` consume Catálogo por `CATALOGO_INTERNAL_URL`,
  HTTP interno y caché en memoria. Nunca introducir credenciales/drivers PostgreSQL.
- Pipeline obligatorio (`app/services/chat.py`): LLM extrae filtros →
  `app/domain/filtros.py` valida y filtra deterministamente → LLM explica solo
  los productos seleccionados (prompts en `app/domain/prompts.py`).
- Clientes Gemini/Groq en `app/repositories`; `app/services/fallback.py`
  Gemini → Groq. Revisar configuración real antes de asumir modelos, cuotas,
  reintentos o tiempos.
- Ante datos ausentes/error, informar degradación; no inventar stock/precios.
- Claves Gemini/Groq solo en backend, nunca variables VITE ni logs.
- Pruebas en `tests/unit` y `tests/integration`; `python -m pytest` y
  `ruff format --check . && ruff check .` con `requirements-dev.txt`. Usar
  dobles de HTTP/proveedores.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Índice breve generado

30 archivos fuente, incluidos los de pruebas. Distribución:
- `app`: 17 archivos.
- `tests`: 13 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
