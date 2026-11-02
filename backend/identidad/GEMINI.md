# Identidad — contexto IA

- Leer `README.md` del servicio y `docs/Guias/GUIA_IDENTIDAD_DESACOPLADA.md`
  desde la raíz. Go/Gin/GORM, puerto 8080, entrada `cmd/identidad/main.go`.
- Rutas y handlers en `internal/routes` y `handlers`; negocio en `services`;
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
- Pruebas: `go test -race ./...`, `go vet ./...` desde este servicio.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Índice breve generado

51 archivos fuente, incluidos los de pruebas. Distribución:
- `cmd`: 1 archivos.
- `internal`: 50 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
