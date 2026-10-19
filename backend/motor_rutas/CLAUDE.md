# Microservicio: Motor de Rutas (FastAPI + OR-Tools + OTP)

Puerto: 8001 | Dockerfile: Dockerfile | Entry: uvicorn app.main:app

## Estructura de módulos

```
app/main.py                              → create_app(): lifespan (pool app_rutas), CORS, probes /_rutas/*
app/config.py, app/auth.py, app/db.py    → entorno, JWT de Identidad (cookie jwt), pool psycopg
app/api/router.py + app/listas/          → CRUD /api/v1/listas (SUP-240), dueño de rutas.*
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

- `GET/POST /api/v1/listas`, `GET/PUT/DELETE /api/v1/listas/{id}` (ver README.md)
- `/_rutas/live`, `/_rutas/ready`, `/health`
- Tests: `python -m unittest discover -s tests -t .`

## Infraestructura OTP

- Motor C++ en contenedor Docker separado (otp-backend:5000)
- Mapa de Temuco descargado via scripts/setup_mapa.sh (Overpass API)
- Perfil: auto (driving)

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Mapa auto-generado: Motor de Rutas (FastAPI + OR-Tools)

**5 archivos .py** detectados


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

### `app/core/strategy.py`

- **class RoutingStrategy(ABC)**
  - `calculate_route(origin: ?, destination: ?) → ?`
  - `optimize_route(origin: ?, waypoints: ?) → ?`

### `app/main.py`

- `root()`
- `health_check()`

### `tests/test_prototipo_ruta_ficticia.py`

- `ejecutar_demo()`
