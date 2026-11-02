# Motor de rutas / Listas — contexto IA

- Leer `README.md`. FastAPI, puerto 8001, `app/main.py`; opcional en Compose.
  No acreditado como desplegado en el inventario de clúster de Sprint 2.
- `app/api/router.py` registra `app/listas/router.py`: CRUD `/api/v1/listas`.
- Configuración en `app/config.py`, sesión en `app/auth.py`, pool/grants en
  `app/db.py`, errores HTTP en `app/errores.py`, persistencia en `app/listas`.
- `RUTAS_DB_URL`/`app_rutas`: CRUD listas y solo lectura de artículos.
  No leer `api.usuarios` ni tablas de Catálogo. No AutoMigrate.
- Sesión solo cookie HttpOnly `jwt` de Identidad. Filtrar siempre por dueño;
  lista ajena/id inválido devuelve 404 sin revelar existencia.
- Borrar lista usa su cascada interna. Agregar/quitar artículos y validar stock
  siguen pendientes: no conceder permisos preventivos.
- `app/algorithms/prototipo_ruta_ficticia.py` es un prototipo TSP, no integración
  real. `otp_ortools_strategy.py` devuelve `pending_implementation`.
- No hay OTP/OSRM operativo ni endpoint de optimización integrado. OTP es Java,
  no C++; `/table/v1/driving` pertenece a OSRM. No reutilizar esa confusión.
- Pruebas: `python -m unittest discover -s tests -t .` desde el servicio.
  Las pruebas PostgreSQL requieren una BD desechable con las variables del README.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Índice breve generado

20 archivos fuente, incluidos los de pruebas. Distribución:
- `app`: 14 archivos.
- `tests`: 6 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
