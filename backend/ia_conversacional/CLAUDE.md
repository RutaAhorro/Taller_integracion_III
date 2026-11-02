# IA conversacional — contexto IA

- FastAPI, puerto 8002, entrada `app/main.py`. Existe en Compose; no afirmar
  despliegue en K8s ni integración completa del frontend.
- `app/routers/chat.py`: `POST /api/v1/chat` con JWT de Identidad en `app/auth.py`.
  Gateway no lo enruta; el frontend todavía llama `/chat/gemini`. No confundir
  esos contratos ni restaurar endpoints antiguos solo para ocultar el desacople.
- `app/productos.py` consume Catálogo por `CATALOGO_INTERNAL_URL`, HTTP interno
  y caché en memoria. Nunca introducir credenciales/drivers PostgreSQL.
- Pipeline obligatorio: LLM extrae filtros → `app/filtros.py` valida/filtra
  deterministamente → LLM explica únicamente los productos seleccionados.
- Proveedores en `app/proveedores`; fallback Gemini → Groq. Revisar configuración
  real antes de asumir modelos, cuotas, reintentos o tiempos disponibles.
- Ante datos ausentes/error, informar degradación; no inventar stock/precios.
- Claves Gemini/Groq solo en backend, nunca variables VITE ni logs.
- Pruebas en `tests/`; ejecutar `python -m pytest` con dependencias de
  `requirements.txt` y `requirements-dev.txt`. Usar dobles de HTTP/proveedores.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Índice breve generado

22 archivos fuente, incluidos los de pruebas. Distribución:
- `app`: 18 archivos.
- `tests`: 4 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
