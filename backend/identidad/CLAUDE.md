# Identidad — contexto IA

- Leer `README.md` del servicio y `.env.example`: configuración, contratos,
  seguridad, operación y correo. Go/Gin/GORM, puerto 8080 por defecto,
  entrada `cmd/identidad/main.go`.
- Rutas y handlers en `internal/handlers`; armado del módulo en `server`; negocio en `services`;
  acceso SQL en `repositories`; sesión en `middleware` y `security`.
- Solo `IDENTITY_DB_URL`/`app_identity`: usuarios, tokens y auditoría autorizados.
  No Catálogo/Control por imports o SQL, ni AutoMigrate.
- Cookie HttpOnly `jwt`, HS256 y expiración obligatoria. Las rutas protegidas
  revalidan estado y rol contra usuarios; no confiar en headers del cliente.
- Roles `registrado`, `colaborador`, `admin`; plan comercial separado del rol.
- Login/registro/Google usan contadores Redis; no desactivar límites para pruebas.
- Conservar validación criptográfica Google, PasswordHash opcional para SSO y
  exigencia de contraseña actual al cambiar correo/contraseña local.
- Admin: no permitir eliminar al último admin activo; registrar auditoría.
- Correo en `internal/mailer`; tokens aleatorios se guardan como hash, no texto.
  Tener servicio de correo no acredita endpoints completos de recuperación.
- Configurar proxies reales, CORS y Secure según entorno; no registrar cookies,
  claves ni tokens. Modo correo `log` solo en desarrollo.
- Configuración validada en `internal/config`, inyectada al arrancar. Conservar
  defaults, nombres compartidos y duración de 24 h de JWT y cookie.
- Pruebas en `tests/unit` y `tests/integration`, fuera de `internal/` y contra
  API pública; no exportar símbolos sólo para testear ni modificar TLS de producción.
- Pruebas: `go test -race ./...`, `go vet ./...` desde este servicio. Los casos
  de BD/Redis reales requieren las variables `IDENTIDAD_TEST_*` y recursos aislados.
- Nombres descriptivos en español para variables, parámetros y receptores
  (`configuracion`, `listaID`, `respuesta`), nunca `c`, `p`, `u` o `l`. Sin
  excepciones por convención del lenguaje: `peticion`/`respuesta` en vez de
  `r`/`w`, `contexto`, `errorObtenido`, `prueba` e `indice`; no todo el equipo
  programa en ese lenguaje.
- Aplicar también a pruebas, scripts Python y Lua embebido. `_` se conserva
  únicamente como descarte. Verificar archivos completos, incluidos los nuevos
  o sin commit; una comparación entre commits no revisa esos cambios.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Índice breve generado

54 archivos fuente, incluidos los de pruebas. Distribución:
- `cmd`: 1 archivos.
- `internal`: 30 archivos.
- `tests`: 23 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
