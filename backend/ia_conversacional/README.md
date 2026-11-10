# IA Conversacional — SUP-344, SUP-345, SUP-353

Microservicio FastAPI independiente (puerto `8002`, punto de entrada `app/main.py`) que implementa un asistente de compras conversacional con un pipeline determinista de 3 etapas diseñado para prevenir alucinaciones de precios, stock y productos.

## Arquitectura y Principios de Diseño

El servicio aplica estrictamente el principio de mínimo privilegio y separación de dominios:

1. **Pipeline Determinista de 3 Etapas:**
   - **Etapa 1 (Extracción estructurada):** El LLM procesa la solicitud del usuario mediante *function calling* forzado para extraer parámetros de búsqueda normalizados (categoría, rango de precios, términos de búsqueda, ordenamiento). Si el modelo no emite *function calling*, el sistema rechaza el texto libre para evitar alucinaciones.
   - **Etapa 2 (Filtrado determinista):** El código del servicio (`app/filtros.py`) aplica los filtros de forma determinista sobre el catálogo de productos reales vigentes con stock.
   - **Etapa 3 (Explicación controlada):** El LLM genera una redacción natural explicativa recibiendo como contexto **únicamente** la lista acotada de productos reales validados en la etapa 2. Si no hay productos que coincidan o si la etapa de explicación falla, se devuelve el resultado filtrado sin interrumpir la respuesta.

2. **Aislamiento de Datos (RNF-05 / SUP-246):**
   - El servicio **no se conecta a PostgreSQL** ni posee credenciales directas de base de datos (`DATABASE_URL`, drivers `psycopg2` o `asyncpg`).
   - Obtiene los productos vigentes mediante HTTP interno hacia el microservicio de Catálogo (`CATALOGO_INTERNAL_URL`).
   - En caso de indisponibilidad o timeout de Catálogo, aplica degradación elegante: mantiene los productos previos en caché de memoria (`fuente: memoria`) o reporta lista vacía (`fuente: sin_datos`), sin interrumpir la API ni inventar datos.

3. **Resiliencia de Proveedores LLM:**
   - Implementa un orquestador de fallback (`app/proveedores/fallback_proveedor.py`): intenta primero con el proveedor primario (**Gemini 3.1 Flash-Lite** en Google AI Studio) con reintentos y backoff exponencial ante errores transitorios (`429`, `503`, timeouts), y conmuta al proveedor secundario (**Groq / Llama 3.1 / 3.3**) si el primario no logra responder.

---

## Endpoints HTTP

| Método | Ruta | Autenticación | Descripción |
|---|---|---|---|
| `GET` | `/health` | Pública | Healthcheck de liveness y estado de caché de catálogo (`status`, `service`, `productos_cargados`, `fuente`: `catalogo`, `memoria` o `sin_datos`). |
| `POST` | `/api/v1/chat` | JWT (Identidad) | Endpoint principal del pipeline conversacional. Valida payload `ChatRequest` y devuelve recomendaciones y filtros. |

### Detalle de `/health`

Responde con el estado del proceso y la fuente del catálogo en memoria:

```json
{
  "status": "ok",
  "service": "ia_conversacional",
  "productos_cargados": 120,
  "fuente": "catalogo"
}
```

Valores posibles de `fuente`:
- `catalogo`: datos sincronizados exitosamente desde el microservicio Catálogo.
- `memoria`: Catálogo no respondió en el último ciclo; se conservan datos previos en caché.
- `sin_datos`: sin conexión con Catálogo y sin datos previos; responde 0 productos sin inventar datos.

### Detalle de `/api/v1/chat`

Protegido mediante token JWT emitido por el servicio de Identidad (algoritmo HMAC-SHA256 con `JWT_SECRET`). Acepta el token mediante:
1. Cookie HttpOnly `jwt` (mecanismo estándar del frontend web).
2. Header `Authorization: Bearer <token>` (fallback para clientes de API o pruebas).

**Request Body (`ChatRequest`):**

```json
{
  "mensaje": "Quiero comprar fideos y salsa de tomate bajo $2000"
}
```

*Restricciones:* `mensaje` obligatorio, longitud entre 1 y 1000 caracteres.

> [!NOTE]
> **Estado de Desacople y Enrutamiento:**  
> La Gateway actualmente **no** enruta el tráfico hacia `/api/v1/chat` (la unificación de rutas en Gateway está programada bajo SUP-278). Los antiguos endpoints experimentales de desarrollo (`/chat/gemini` y `/chat/groq`) fueron completamente retirados. El frontend o las pruebas interactúan directamente contra el puerto `8002` (vía `VITE_IA_URL`).

---

## Configuración y Variables de Entorno

El servicio lee su configuración de variables de entorno (o desde un archivo `.env` en la raíz).

### Variables Compartidas (SUP-345)

| Variable | Valor por defecto | Obligatoria | Propósito y Formato |
|---|---|---|---|
| `JWT_SECRET` | *(Sin fallback)* | **Sí** | Secreto de firma y validación de tokens JWT de usuario (HMAC-SHA256). Mínimo 32 caracteres (generar con `openssl rand -hex 32`). Debe coincidir con Identidad. |
| `IA_CORS_ALLOWED_ORIGINS` | `http://localhost:3000,http://localhost:5173` | No | Lista separada por comas de orígenes exactos permitidos (sin comodín `*` ni barra final). Fallback a `CORS_ALLOWED_ORIGINS`. |
| `CATALOGO_INTERNAL_URL` | `http://catalogo:8080` | No | Origen HTTP interno limpio hacia el microservicio Catálogo (sin path). En ejecución local usar `http://localhost:8080` o `http://localhost:8083`. |
| `GEMINI_API_KEY` | *(Vacío)* | Condicional | Clave de API de Google AI Studio para el proveedor primario (Gemini). |
| `GROQ_API_KEY` | *(Vacío)* | Condicional | Clave de API de Groq Cloud para el proveedor secundario de resiliencia (Groq). |

### Variables Específicas del Servicio

| Variable | Valor por defecto | Obligatoria | Propósito |
|---|---|---|---|
| `CATALOGO_TIMEOUT` | `5.0` | No | Tiempo máximo de espera (segundos) para consultas HTTP hacia Catálogo. |
| `IA_CATALOGO_CACHE_TTL` | `60.0` | No | Tiempo de vida de la caché local de productos en memoria (segundos). |
| `IA_CATALOGO_PAGE_LIMIT` | `100` | No | Cantidad de productos solicitados por página al endpoint de Catálogo. |
| `IA_CATALOGO_MAX_PAGINAS` | `10` | No | Límite de seguridad de páginas a recorrer desde Catálogo. |
| `GROQ_ENDPOINT` | `https://api.groq.com/openai/v1/chat/completions` | No | Endpoint de API de inferencia de Groq. |
| `GROQ_MODEL_EXTRACCION` | `llama-3.1-8b-instant` | No | Modelo Groq para extracción estructurada de filtros. |
| `GROQ_MODEL_EXPLICACION` | `llama-3.3-70b-versatile` | No | Modelo Groq para redacción de explicación de recomendaciones. |
| `IA_GROQ_TIMEOUT` | `15.0` | No | Timeout de llamadas HTTP hacia Groq (segundos). |
| `IA_GROQ_MAX_TOKENS_EXTRACCION` | `512` | No | Límite de tokens de salida para extracción de filtros en Groq. |
| `IA_GROQ_MAX_TOKENS_EXPLICACION` | `200` | No | Límite de tokens de salida para la explicación en Groq. |
| `IA_GROQ_FREQUENCY_PENALTY` | `0.4` | No | Penalización de frecuencia en Groq para evitar repeticiones. |
| `GEMINI_MODEL` | `gemini-3.1-flash-lite` | No | Modelo de Gemini utilizado en Google AI Studio. |
| `GEMINI_API_BASE_URL` | `https://generativelanguage.googleapis.com/v1beta` | No | URL base de la API REST de Google Generative Language. |
| `IA_GEMINI_TIMEOUT` | `15.0` | No | Timeout de llamadas HTTP hacia Gemini (segundos). |
| `IA_GEMINI_THINKING_LEVEL` | `low` | No | Nivel de pensamiento en Gemini 3.1 Flash-Lite para minimizar latencia. |
| `IA_GEMINI_MAX_TOKENS_EXTRACCION` | `1200` | No | Límite de tokens de salida para extracción en Gemini. |
| `IA_GEMINI_MAX_TOKENS_EXPLICACION` | `2048` | No | Límite de tokens de salida para explicación en Gemini. |
| `IA_FALLBACK_RETRIES_PRIMARIO` | `4` | No | Reintentos máximos del proveedor primario ante fallos recuperables. |
| `IA_FALLBACK_RETRIES_SECUNDARIO` | `1` | No | Reintentos máximos del proveedor secundario antes de fallar. |
| `IA_FALLBACK_BACKOFF_BASE` | `1.5` | No | Multiplicador base (segundos) para el cálculo de backoff exponencial. |
| `IA_FALLBACK_JITTER_MAX` | `0.5` | No | Variación aleatoria máxima (segundos) agregada al backoff. |
| `IA_CHAT_MAX_MENSAJE_LEN` | `1000` | No | Longitud máxima permitida en caracteres para el mensaje del usuario. |
| `IA_EXPLICACION_MAX_PRODUCTOS` | `8` | No | Cantidad máxima de productos entregados en el prompt de explicación. |

---

## Ejecución Local

### Con `uv` (Recomendado)

Desde el directorio `backend/ia_conversacional`:

```bash
# Ejecución del servidor de desarrollo con recarga automática
JWT_SECRET=$(openssl rand -hex 32) \
CATALOGO_INTERNAL_URL=http://localhost:8083 \
uv run --with-requirements requirements.txt uvicorn app.main:app --host 0.0.0.0 --port 8002 --reload
```

### Con entorno virtual tradicional (`venv`)

```bash
cd backend/ia_conversacional
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

export JWT_SECRET=$(openssl rand -hex 32)
export CATALOGO_INTERNAL_URL=http://localhost:8083
uvicorn app.main:app --host 0.0.0.0 --port 8002 --reload
```

Verificación en terminal:

```bash
curl -fsS http://localhost:8002/health
```

---

## Ejecución con Docker y Docker Compose

### Construcción individual de la imagen

Desde la raíz del repositorio:

```bash
docker build -t ia-conversacional backend/ia_conversacional
```

La imagen utiliza `python:3.11-slim`, ejecuta como usuario sin privilegios `65532:65532`, expone el puerto `8002` e incluye un healthcheck nativo que consulta `http://127.0.0.1:8002/health`.

### Despliegue con Docker Compose

El servicio está integrado en el `docker-compose.yml` principal bajo la red `microservices_net`, con sistema de archivos de solo lectura (`read_only: true`), límite de recursos (0.5 CPUs, 300M RAM) y dependencia saludable de `catalogo`:

```bash
# Levantar el servicio y sus dependencias
docker compose up -d ia_conversacional

# Consultar logs en tiempo real
docker compose logs -f ia_conversacional

# Verificar estado de salud
docker compose ps ia_conversacional
```

---

## Pruebas Automatizadas

La suite de pruebas utiliza `pytest` y dobles de prueba HTTP (`httpx.Client` / `FastAPI TestClient`) sin realizar llamadas reales a APIs externas de pago ni requerir Catálogo activo.

### Ejecución de pruebas con `uv`

Desde `backend/ia_conversacional`:

```bash
uv run --with-requirements requirements.txt --with-requirements requirements-dev.txt pytest tests/
```

### Ejecución con `pytest` en entorno virtual

```bash
# Con el entorno virtual activo y requirements-dev.txt instalado:
pytest tests/

# Ejecutar por suite o categoría:
pytest tests/unit/
pytest tests/integration/

# Ejecutar por módulo de prueba:
pytest tests/unit/test_productos.py -v
pytest tests/integration/test_chat_auth.py -v
```
