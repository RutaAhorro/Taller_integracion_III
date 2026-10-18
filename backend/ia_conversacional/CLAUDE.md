# Microservicio: IA Conversacional (FastAPI + Groq/Gemini)

Puerto: 8002 | Dockerfile: Dockerfile | Entry: uvicorn app.main:app

## Dependencias entre módulos

```
app/main.py ──→ config, ia_definiciones, productos, utils
               ├──→ proveedores/gemini_proveedor, groq_proveedor, fallback_proveedor
               └──→ routers/chat, routers/productos

app/routers/chat.py ──→ explicacion, filtros, productos, proveedores/base, schemas
app/routers/productos.py ──→ productos

app/proveedores/gemini_proveedor.py ──→ explicacion, filtros, proveedores/base
app/proveedores/groq_proveedor.py ──→ explicacion, filtros, proveedores/base
app/proveedores/fallback_proveedor.py ──→ proveedores/base
```

## Clases y firmas

- `ProveedorIA(ABC)` [app/proveedores/base.py]: interfaz base
  - `async extraer_filtros(mensaje: str) → ResultadoExtraccion`
  - `async generar_explicacion(mensaje_usuario: str, productos_filtrados: list) → str`
- `ResultadoExtraccion` [app/proveedores/base.py]: dataclass
  - campos: filtros, texto_libre, error, error_estado, nota, extra, tiempo_extraccion_ms
- `GeminiProveedor(ProveedorIA)` [app/proveedores/gemini_proveedor.py]
  - `__init__(api_key, endpoint, modelo, definicion_funcion)`
  - Usa functionCallingConfig mode=ANY, thinkingLevel=low
- `GroqProveedor(ProveedorIA)` [app/proveedores/groq_proveedor.py]
  - `__init__(api_key, endpoint, modelo_extraccion, modelo_explicacion, definicion_funcion)`
  - Extracción: llama-3.1-8b-instant, Explicación: llama-3.3-70b-versatile
- `FallbackProveedor(ProveedorIA)` [app/proveedores/fallback_proveedor.py]
  - `__init__(proveedores: list, max_retries, backoff_base, timeout)`
  - Reintentos exponenciales con jitter, detección de errores recuperables (429, 5xx)
- `ProductoRepositorio` [app/productos.py]
  - `__init__(ruta_json: Path)`, `obtener_todos() → list`, `__len__() → int`
- `ChatRequest(BaseModel)` [app/schemas.py]: campo `mensaje: str`
- `RespuestaUTF8(JSONResponse)` [app/utils.py]: media_type utf-8

## Funciones clave

- `filtrar_productos(catalogo, filtros) → list` [app/filtros.py]: filtrado determinista por precio, categoría, distancia
- `sanitizar_filtros_crudos(crudo: dict) → dict` [app/filtros.py]: sanea tipos de filtros del LLM
- `rescatar_filtros_de_error_groq(response) → dict|None` [app/filtros.py]: recupera JSON de errores 400
- `construir_prompt_explicacion(mensaje, productos) → str` [app/explicacion.py]: inyecta top 8 productos
- `generar_explicacion_con_tiempo(generador, mensaje, productos) → (str|None, int)` [app/explicacion.py]

## Rutas FastAPI

- `POST /chat/groq` → pipeline 3 etapas via Groq
- `POST /chat/gemini` → pipeline 3 etapas via Gemini
- `GET /productos` → catálogo completo desde productos.json
- `GET /` → health check

## Configuración [app/config.py]

- GROQ_MODEL_EXTRACCION = llama-3.1-8b-instant
- GROQ_MODEL_EXPLICACION = llama-3.3-70b-versatile
- GEMINI_MODEL = gemini-3.1-flash-lite
- API keys desde variables de entorno (.env)

## Pipeline anti-alucinación (RESPETAR SIEMPRE)

1. LLM extrae filtros via function calling → `extraer_filtros()`
2. Python filtra catálogo → `filtrar_productos()` (NUNCA el LLM)
3. LLM explica resultados → `generar_explicacion()` (SOLO datos de paso 2)

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Mapa auto-generado: IA Conversacional (FastAPI)

**17 archivos .py** detectados


### `app/config.py`


### `app/explicacion.py`

- `construir_prompt_explicacion(mensaje_usuario: str, productos_filtrados: list) → str`
- `async generar_explicacion_con_tiempo(generador, mensaje_usuario: str, productos_filtrados: list)`

### `app/filtros.py`

- `sanitizar_filtros_crudos(crudo: dict) → dict`
- `rescatar_filtros_de_error_groq(response: httpx.Response) → ?`
- `_extraer_json_de_texto(texto: str) → ?`
- `_tokenizar_categoria(texto: str) → list`
- `_texto_contiene(a: str, b: str) → bool`
- `_coincide_por_texto(frase: str, producto: dict) → bool`
- `filtrar_productos(catalogo: list, filtros: dict) → list`

### `app/ia_definiciones.py`


### `app/main.py`

- Imports internos: app, app.ia_definiciones, app.productos, app.proveedores.fallback_proveedor, app.proveedores.gemini_proveedor, app.proveedores.groq_proveedor, app.routers.chat, app.routers.productos, app.utils
- `health_check()`

### `app/productos.py`

- **class ProductoRepositorio**
  - `__init__(ruta_json: Path)`
  - `_cargar() → list`
  - `obtener_todos() → list`
  - `__len__() → int`

### `app/proveedores/base.py`

- **class ResultadoExtraccion**
- **class ProveedorIA(ABC)**
  - `async extraer_filtros(mensaje: str) → ResultadoExtraccion`
  - `async generar_explicacion(mensaje_usuario: str, productos_filtrados: list) → str`

### `app/proveedores/fallback_proveedor.py`

- Imports internos: app.proveedores.base
- **class FallbackProveedor(ProveedorIA)**
  - `__init__(proveedores: ?, max_retries_primario: int, max_retries_secundario: int, backoff_base: float, timeout: float)`
  - `_es_error_recuperable(error: Exception) → bool`
  - `async _ejecutar_con_reintentos(proveedor: ProveedorIA, metodo: str) → any`
  - `async extraer_filtros(mensaje: str) → ResultadoExtraccion`
  - `async generar_explicacion(mensaje_usuario: str, productos_filtrados: list) → str`

### `app/proveedores/gemini_proveedor.py`

- Imports internos: app.explicacion, app.filtros, app.proveedores.base
- **class GeminiProveedor(ProveedorIA)**
  - `__init__(api_key: ?, endpoint: str, modelo: str, definicion_funcion: dict)`
  - `_headers() → dict`
  - `async extraer_filtros(mensaje: str) → ResultadoExtraccion`
  - `async generar_explicacion(mensaje_usuario: str, productos_filtrados: list) → str`

### `app/proveedores/groq_proveedor.py`

- Imports internos: app.explicacion, app.filtros, app.proveedores.base
- **class GroqProveedor(ProveedorIA)**
  - `__init__(api_key: ?, endpoint: str, modelo_extraccion: str, modelo_explicacion: str, definicion_funcion: dict)`
  - `_headers() → dict`
  - `async extraer_filtros(mensaje: str) → ResultadoExtraccion`
  - `async generar_explicacion(mensaje_usuario: str, productos_filtrados: list) → str`

### `app/routers/chat.py`

- Imports internos: app.explicacion, app.filtros, app.productos, app.proveedores.base, app.schemas
- `crear_router_chat(repositorio: ProductoRepositorio, proveedor_groq: ProveedorIA, proveedor_gemini: ProveedorIA) → APIRouter`

### `app/routers/productos.py`

- Imports internos: app.productos
- `crear_router_productos(repositorio: ProductoRepositorio) → APIRouter`

### `app/schemas.py`

- **class ChatRequest(BaseModel)**

### `app/utils.py`

- **class RespuestaUTF8(JSONResponse)**
- `fix_encoding_consola() → ?`
