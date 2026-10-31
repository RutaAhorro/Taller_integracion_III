# Microservicio: Motor de Rutas (FastAPI + OR-Tools + OTP)

Puerto: 8001 | Dockerfile: Dockerfile | Entry: uvicorn app.main:app

## Estructura de módulos

```
app/main.py                              → FastAPI app, GET / (health check)
app/core/strategy.py                     → RoutingStrategy(ABC) interfaz base
app/algorithms/otp_ortools_strategy.py  → OTPOrToolsStrategy(RoutingStrategy)
app/algorithms/prototipo_ruta_ficticia.py → Prototipo standalone con datos ficticios
tests/test_prototipo_ruta_ficticia.py    → Test del prototipo
```

## Patrón de diseño: Strategy Pattern

- `RoutingStrategy(ABC)` [app/core/strategy.py]: contrato base
  - `calculate_route(origin, destination) → Dict`
  - `optimize_route(origin, waypoints) → Dict`
- `OTPOrToolsStrategy(RoutingStrategy)` [app/algorithms/otp_ortools_strategy.py]
  - `__init__(otp_base_url="http://otp-backend:5000")`
  - Esqueleto: consulta OTP para matriz de distancias, OR-Tools para TSP

## Prototipo funcional [app/algorithms/prototipo_ruta_ficticia.py]

- `haversine(lat1, lon1, lat2, lon2) → float`: distancia esférica en km
- `crear_coordenadas() → Dict`: puntos ficticios en Temuco (Jumbo, Lider, etc.)
- `calcular_matriz_distancias(coordenadas) → (ndarray, List[str])`
- `resolver_tsp(matriz) → (orden, distancia, tiempo)`: usa RoutingIndexManager + GUIDED_LOCAL_SEARCH

## Dependencias (requirements.txt)

- fastapi, uvicorn, pydantic
- ortools >= 9.7.0 (TSP solver)
- httpx (cliente HTTP async para OTP)
- SIN osmnx ni networkx (eliminados por OOM en hardware limitado)

## Rutas FastAPI

- `GET /` → mensaje de servicio activo
- Router de API comentado (app/api/router.py no implementado aún)

## Infraestructura OTP

- Motor C++ en contenedor Docker separado (otp-backend:5000)
- Mapa de Temuco descargado via scripts/setup_mapa.sh (Overpass API)
- Perfil: auto (driving)

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Mapa auto-generado: Motor de Rutas (FastAPI + OR-Tools)

**20 archivos .py** detectados


### `app/algorithms/otp_ortools_strategy.py`

- Imports internos: app.core.strategy
- **class OTPOrToolsStrategy(RoutingStrategy)**
  - `__init__(otp_base_url: str)`
  - `calculate_route(origin: ?, destination: ?) → ?`
  - `optimize_route(origin: ?, waypoints: ?) → ?`

### `app/algorithms/prototipo_ruta_ficticia.py`

- `haversine(lat1: float, lon1: float, lat2: float, lon2: float) → float`
- `crear_coordenadas() → ?`
- `calcular_matriz_distancias(coordenadas: ?) → ?`
- `resolver_tsp(matriz_distancias: np.ndarray) → ?`
- `mostrar_resultados(orden: ?, nombres: ?, matriz_distancias: np.ndarray) → ?`
- `main() → ?`

### `app/api/router.py`

- Imports internos: app.listas.router

### `app/auth.py`

- Imports internos: app.errores
- **class Sesion**
- `validar_token(token: str, secreto: str) → Sesion`
- `sesion_actual(request: Request) → Sesion`

### `app/config.py`

- **class ErrorConfiguracion(ValueError)**
- **class Configuracion**
- `cargar(entorno: ?) → Configuracion`

### `app/core/strategy.py`

- **class RoutingStrategy(ABC)**
  - `calculate_route(origin: ?, destination: ?) → ?`
  - `optimize_route(origin: ?, waypoints: ?) → ?`

### `app/db.py`

- **class ErrorAlmacenamiento(RuntimeError)**
- `async abrir_pool(dsn: str, maximo: int) → AsyncConnectionPool`
- `async verificar_permisos(pool: AsyncConnectionPool) → ?`
- `async ping(pool: AsyncConnectionPool, plazo: float) → bool`

### `app/errores.py`

- **class ErrorAPI(Exception)**
  - `__init__(estado: int, mensaje: str, detalle: ?) → ?`
- `respuesta_error(estado: int, mensaje: str, detalle: ?, cabeceras: ?) → JSONResponse`
- `registrar_manejadores(app: FastAPI) → ?`

### `app/listas/modelos.py`

- **class ListaCrear(BaseModel)**
  - `_nombre(cls, valor: str) → str`
- **class ListaActualizar(BaseModel)**
  - `_nombre(cls, valor: ?) → ?`
  - `_algun_cambio() → ListaActualizar`
- **class Articulo(BaseModel)**
- **class Lista(BaseModel)**
- **class ListaDetalle(Lista)**
- **class RespuestaListas(BaseModel)**
- `normalizar_nombre(valor: str) → str`

### `app/listas/repositorio.py`

- Imports internos: app.listas.modelos
- **class RepositorioListas(Protocol)**
  - `async contar(usuario_id: str) → int`
  - `async listar(usuario_id: str, limite: int) → ?`
  - `async crear(usuario_id: str, nombre: str) → Lista`
  - `async obtener(usuario_id: str, lista_id: str) → ?`
  - `async actualizar(usuario_id: str, lista_id: str, nombre: ?, estado: ?) → ?`
  - `async eliminar(usuario_id: str, lista_id: str) → bool`
- **class RepositorioListasPostgres**
  - `__init__(pool: AsyncConnectionPool) → ?`
  - `async _una(consulta: str, parametros: tuple) → ?`
  - `async contar(usuario_id: str) → int`
  - `async listar(usuario_id: str, limite: int) → ?`
  - `async crear(usuario_id: str, nombre: str) → Lista`
  - `async obtener(usuario_id: str, lista_id: str) → ?`
  - `async actualizar(usuario_id: str, lista_id: str, nombre: ?, estado: ?) → ?`
  - `async eliminar(usuario_id: str, lista_id: str) → bool`
- `_utc(valor: datetime) → datetime`
- `_lista(fila: dict) → Lista`

### `app/listas/router.py`

- Imports internos: app.auth, app.errores, app.listas.modelos, app.listas.repositorio
- `repositorio(request: Request) → RepositorioListas`
- `_id_o_404(lista_id: str) → str`
- `async listar(sesion: Sesion, repo: RepositorioListas) → RespuestaListas`
- `async crear(datos: ListaCrear, sesion: Sesion, repo: RepositorioListas) → Lista`
- `async obtener(lista_id: str, sesion: Sesion, repo: RepositorioListas) → ListaDetalle`
- `async actualizar(lista_id: str, datos: ListaActualizar, sesion: Sesion, repo: RepositorioListas) → Lista`
- `async eliminar(lista_id: str, sesion: Sesion, repo: RepositorioListas) → Response`

### `app/main.py`

- Imports internos: app, app.api.router, app.config, app.errores, app.listas.repositorio
- `async _almacenamiento(app: FastAPI, config: Configuracion) → ?`
- `create_app(config: ?, repositorio: ?) → FastAPI`
- `_configurar_logs() → ?`

### `tests/apoyo.py`

- Imports internos: app.config, app.listas.modelos
- **class RepositorioMemoria**
  - `__init__() → ?`
  - `_lista(fila: dict) → Lista`
  - `_propia(usuario_id: str, lista_id: str) → ?`
  - `async contar(usuario_id: str) → int`
  - `async listar(usuario_id: str, limite: int) → ?`
  - `async crear(usuario_id: str, nombre: str) → Lista`
  - `async obtener(usuario_id: str, lista_id: str) → ?`
  - `async actualizar(usuario_id: str, lista_id: str, nombre: ?, estado: ?) → ?`
  - `async eliminar(usuario_id: str, lista_id: str) → bool`
- `token(usuario_id: ?, secreto: str, algoritmo: str) → str`

### `tests/test_config.py`

- Imports internos: app.config
- **class ConfiguracionTest(unittest.TestCase)**
  - `test_valida()`
  - `test_rechaza_sin_exponer_secretos()`

### `tests/test_listas.py`

- Imports internos: app.main
- **class ListasTest(unittest.TestCase)**
  - `setUp() → ?`
  - `como(usuario_id: str) → ?`
  - `crear(nombre: str) → dict`
  - `test_crud_completo_del_dueno()`
  - `test_listas_ajenas_responden_404_sin_modificarse()`
  - `test_id_mal_formado_es_404()`
  - `test_validacion_con_envoltorio_y_sin_eco_del_valor()`
  - `test_json_mal_formado_usa_envoltorio()`
  - `test_nombre_sin_caracteres_de_control()`
  - `test_tope_de_listas_por_usuario()`
  - `test_rutas_y_metodos_desconocidos_usan_envoltorio()`
- **class AutenticacionTest(unittest.TestCase)**
  - `setUp() → ?`
  - `estado_con(valor: ?) → int`
  - `test_token_valido()`
  - `test_rechaza_tokens_inseguros()`
  - `test_ignora_identidad_en_headers()`
- **class InfraestructuraHTTPTest(unittest.TestCase)**
  - `setUp() → ?`
  - `test_cors_lista_blanca()`
  - `test_request_id_se_propaga_o_genera()`
  - `test_probes()`

### `tests/test_prototipo_ruta_ficticia.py`

- `ejecutar_demo()`

### `tests/test_repositorio_postgres.py`

- Imports internos: app, app.listas.repositorio, app.main
- **class RepositorioPostgresTest(unittest.IsolatedAsyncioTestCase)**
  - `async asyncSetUp() → ?`
  - `async test_permisos_y_crud_aislado_por_usuario()`
  - `async test_detalle_con_articulos_y_borrado_en_cascada()`
  - `async test_app_rutas_no_sale_de_su_matriz()`
