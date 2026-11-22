# IA Conversacional

Microservicio FastAPI (puerto `8002`, entrada `app.main:app`) que responde
pedidos de compra en lenguaje natural con un pipeline de tres etapas pensado
para no inventar productos, precios ni stock.

> [!IMPORTANT]
> **Estado de despliegue:** el servicio existe en `docker-compose.yml` y el
> Gateway enruta `/api/v1/chat` si se define `IA_URL`, pero **no figura como
> servicio desplegado en el inventario del Sprint 2** y no tiene manifiestos en
> `infrastructure/k8s/`. Que las pruebas pasen no acredita integración ni
> despliegue.

## Pipeline

1. **Extracción (LLM)** — función declarada en `app/domain/prompts.py`. El
   modelo debe llamar `extraer_filtros_busqueda` (function calling forzado) y
   devuelve `precio_max`, `categoria`, `distancia_max` y `palabras_clave`.
   `app/domain/filtros.py` descarta los campos mal tipados. Si responde texto
   libre, el chat lo ignora.
2. **Filtrado determinista** — `app/domain/filtros.py`. El código filtra los
   productos reales de Catálogo por precio máximo, categoría (texto contra
   categoría o nombre) y distancia, y ordena por precio ascendente.
   `palabras_clave` se extrae pero hoy no filtra.
3. **Explicación (LLM)** — prompt en `app/domain/prompts.py`, ejecución en
   `app/services/chat.py`. El modelo redacta solo
   con los productos ya filtrados (máximo `IA_EXPLICACION_MAX_PRODUCTOS`). Si
   falla, se responde con los productos y un texto de respaldo.

**Proveedores**: Gemini primario y Groq secundario (`app/repositories`). El
respaldo (`app/services/fallback.py`) reintenta con backoff exponencial los errores
transitorios (red, timeout, HTTP 429/500/502/503/504) y luego cambia de
proveedor.

**Catálogo** (`app/repositories/catalogo.py`): solo HTTP interno a
`GET {CATALOGO_INTERNAL_URL}/api/v1/productos?en_stock=true&page=&limit=`, con
caché en memoria. El servicio no tiene drivers ni credenciales de PostgreSQL.
Si Catálogo falla, conserva los productos previos (`fuente: memoria`) o informa
`sin_datos`.

### Limitaciones conocidas

- Catálogo no entrega distancia: `distancia_km` queda en `0.0` y `distancia_max`
  no descarta productos (SUP-315).
- El filtrado por categoría compara en minúsculas sin normalizar tildes.
- Los errores HTTP de Gemini y Groq en la extracción se devuelven como resultado
  con `error` en vez de excepción, así que el respaldo no reintenta ni cambia de
  proveedor en esa etapa (SUP-315).

## Estructura

Misma organización por capas que Identidad (`api`, `services`,
`repositories`, `domain`, `config`), con un archivo por tema.

```
app/
  main.py            Arma la aplicación, CORS, /health y el router
  config.py          Lectura y validación del entorno al arrancar
  api/
    chat.py          POST /api/v1/chat: cuerpo, sesión, límite y 503
    sesion.py        Validación JWT de Identidad y límite por usuario
  services/
    chat.py          Orquestación de las tres etapas y texto de respaldo
    fallback.py      Reintentos y cambio de proveedor
  repositories/
    catalogo.py      Cliente HTTP de Catálogo con caché y degradación
    gemini.py        Cliente REST de Gemini
    groq.py          Cliente REST de Groq y rescate de errores de esquema
  domain/
    filtros.py       Saneamiento de filtros y filtrado determinista
    prompts.py       Función de extracción y prompt de explicación
    proveedores.py   Interfaz ProveedorIA y ResultadoExtraccion
tests/
  unit/              Configuración, filtros, Catálogo, respaldo y límite
  integration/       /health, autenticación y endpoint de chat con dobles
```

## Endpoints

| Método | Ruta | Autenticación | Descripción |
|---|---|---|---|
| `GET` | `/health` | Pública | `status`, `service`, `productos_cargados` y `fuente` (`catalogo`, `memoria` o `sin_datos`). No llama a la red. |
| `POST` | `/api/v1/chat` | JWT de Identidad | Ejecuta el pipeline. Cuerpo `{"mensaje": "..."}` (1 a `IA_CHAT_MAX_MENSAJE_LEN` caracteres). |

El JWT se acepta en la cookie HttpOnly `jwt` (navegador) o en
`Authorization: Bearer <token>`, firmado con HS256 y `JWT_SECRET`, con los
claims `exp` y `user_id` obligatorios.

Respuesta 200 de `/api/v1/chat`:

```json
{
  "respuesta": "texto para el usuario",
  "filtros_aplicados": {"categoria": "verduras", "precio_max": 2000},
  "productos_recomendados": [{"id": "...", "nombre": "...", "precio": 890.0}],
  "tiempo_extraccion_ms": 420,
  "tiempo_explicacion_ms": 610
}
```

`filtros_aplicados` es `null` si el modelo no extrajo filtros. Errores: `401`
sin sesión válida, `422` mensaje inválido, `429` con `Retry-After` al exceder
el límite por usuario y `503` si ningún proveedor pudo extraer filtros.

### Cambios de contrato (SUP-387)

El frontend solo lee `respuesta`; ningún consumidor del repositorio lee las
claves retiradas.

| Antes | Ahora |
|---|---|
| `filtros` duplicaba `filtros_aplicados` | Retirado: usar `filtros_aplicados` |
| `productos` duplicaba `productos_recomendados` | Retirado: usar `productos_recomendados` |
| Con catálogo vacío faltaban `tiempo_extraccion_ms` y `tiempo_explicacion_ms` | Siempre presentes (`0` si la etapa no corrió) |

## Configuración

Todas las variables tienen un ejemplo en [`.env.example`](.env.example). Las
compartidas siguen [la guía de configuración](../../docs/Guias/GUIA_CONFIGURACION.md).

### Compartidas

| Variable | Defecto | Obligatoria | Uso |
|---|---|---|---|
| `JWT_SECRET` | — | **Sí** | Verificación HS256 de la sesión de Identidad; mínimo 32 caracteres. |
| `IA_CORS_ALLOWED_ORIGINS` | `http://localhost:3000,http://localhost:5173` | No | Orígenes exactos, sin `*` ni barra final. Respaldo: `CORS_ALLOWED_ORIGINS`. |
| `CATALOGO_INTERNAL_URL` | `http://catalogo:8080` | No | Origen HTTP de Catálogo, sin path ni credenciales. |
| `GEMINI_API_KEY` | vacío | Condicional | Clave del proveedor primario. Solo backend. |
| `GROQ_API_KEY` | vacío | Condicional | Clave del proveedor secundario. Solo backend. |

Sin ninguna clave el servicio arranca con una advertencia, y el chat responde 503.

### Propias del servicio

| Variable | Defecto | Uso |
|---|---|---|
| `CATALOGO_TIMEOUT` | `5.0` | Timeout (s) hacia Catálogo. |
| `IA_CATALOGO_CACHE_TTL` | `60.0` | Vida (s) de la caché de productos; `0` consulta siempre. |
| `IA_CATALOGO_PAGE_LIMIT` | `100` | Productos por página pedidos a Catálogo. |
| `IA_CATALOGO_MAX_PAGINAS` | `10` | Páginas máximas; si hay más, se registra el truncado. |
| `GROQ_ENDPOINT` | `https://api.groq.com/openai/v1/chat/completions` | Endpoint de Groq. |
| `GROQ_MODEL_EXTRACCION` | `llama-3.1-8b-instant` | Modelo Groq de la etapa 1. |
| `GROQ_MODEL_EXPLICACION` | `llama-3.3-70b-versatile` | Modelo Groq de la etapa 3. |
| `IA_GROQ_TIMEOUT` | `15.0` | Timeout (s) hacia Groq. |
| `IA_GROQ_MAX_TOKENS_EXTRACCION` | `512` | Tokens de salida, etapa 1. |
| `IA_GROQ_MAX_TOKENS_EXPLICACION` | `200` | Tokens de salida, etapa 3. |
| `IA_GROQ_FREQUENCY_PENALTY` | `0.4` | Penalización de repetición. |
| `GEMINI_MODEL` | `gemini-3.1-flash-lite` | Modelo Gemini. |
| `GEMINI_API_BASE_URL` | `https://generativelanguage.googleapis.com/v1beta` | Base de la API de Gemini. |
| `GEMINI_ENDPOINT` | derivado de base y modelo | Endpoint completo; si se define, anula `GEMINI_MODEL` (se advierte al arrancar). |
| `IA_GEMINI_TIMEOUT` | `15.0` | Timeout (s) hacia Gemini. |
| `IA_GEMINI_THINKING_LEVEL` | `low` | Nivel de razonamiento; `low` es el mínimo de la serie 3. |
| `IA_GEMINI_MAX_TOKENS_EXTRACCION` | `1200` | Tokens de salida, etapa 1. |
| `IA_GEMINI_MAX_TOKENS_EXPLICACION` | `2048` | Tokens de salida, etapa 3. |
| `IA_FALLBACK_RETRIES_PRIMARIO` | `4` | Reintentos del primario. |
| `IA_FALLBACK_RETRIES_SECUNDARIO` | `1` | Reintentos del secundario. |
| `IA_FALLBACK_BACKOFF_BASE` | `1.5` | Espera base (s) del backoff exponencial. |
| `IA_FALLBACK_JITTER_MAX` | `0.5` | Variación aleatoria máxima (s) sumada a cada espera. |
| `IA_CHAT_MAX_MENSAJE_LEN` | `1000` | Largo máximo del mensaje. |
| `IA_EXPLICACION_MAX_PRODUCTOS` | `8` | Productos enviados al prompt de explicación. |
| `IA_CHAT_RATE_LIMIT_SOLICITUDES` | `10` | Mensajes por usuario en la ventana. |
| `IA_CHAT_RATE_LIMIT_VENTANA_SEGUNDOS` | `60.0` | Ventana deslizante (s). |

El límite por usuario vive en memoria de cada réplica: con N réplicas, el
límite efectivo es N veces la cuota.

## Ejecución

```bash
cd backend/ia_conversacional
python -m venv .venv && source .venv/bin/activate
pip install -r requirements-dev.txt
export JWT_SECRET=$(openssl rand -hex 32) CATALOGO_INTERNAL_URL=http://localhost:8083
uvicorn app.main:app --host 0.0.0.0 --port 8002 --reload
curl -fsS http://localhost:8002/health
```

Con Docker (imagen `python:3.11-slim`, usuario `65532:65532`, healthcheck sobre
`/health`):

```bash
docker build -t ia-conversacional backend/ia_conversacional
docker compose up -d ia_conversacional
```

## Pruebas

Las pruebas usan dobles de HTTP y de proveedores: no llaman a Gemini, Groq ni
Catálogo. El CI (`.github/workflows/ia-conversacional-checks.yml`) ejecuta
`pytest tests/` y construye la imagen.

```bash
pip install -r requirements-dev.txt
python -m pytest            # tests/unit y tests/integration
ruff format --check . && ruff check .
```
