# Identidad — contexto IA

- Leer `README.md` del servicio y `.env.example`: configuración, contratos,
  seguridad y operación. Go/Gin/GORM, puerto 8080 por defecto,
  entrada `cmd/identidad/main.go`.
- HTTP en `internal/api` (router, handlers, sesión JWT, RBAC, límites y errores);
  negocio en `services`; SQL y verificación de permisos en `repositories`;
  entidades y contratos en `domain`; un archivo por tema, sin paquetes de un archivo.
- Solo `IDENTITY_DB_URL`/`app_identity`: usuarios, tokens y auditoría autorizados.
  No Catálogo/Control por imports o SQL, ni AutoMigrate.
- Cookie HttpOnly `jwt`, HS256 y expiración obligatoria. Las rutas protegidas
  revalidan estado y rol contra usuarios; no confiar en headers del cliente.
- Direcciones del usuario (`api.direcciones_usuario`, migración 11): el dueño sale
  sólo de la sesión; ajenas responden 404; una principal por usuario (índice único
  parcial + advisory lock); límite 10 por usuario.
- Roles `registrado`, `colaborador`, `admin`, garantizados por `usuarios_rol_check`
  (migración 10); plan comercial separado del rol.
- Login/registro/Google usan contadores Redis; no desactivar límites para pruebas.
- Conservar validación criptográfica Google, PasswordHash opcional para SSO y
  exigencia de contraseña actual al cambiar correo/contraseña local.
- Admin: no permitir eliminar al último admin activo; registrar auditoría.
- No hay recuperación de contraseña ni correo: se podaron por no tener endpoints.
  Al reimplementarla, guardar tokens aleatorios sólo como hash.
- Configurar proxies reales, CORS y Secure según entorno; no registrar cookies,
  claves ni tokens.
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

34 archivos fuente, incluidos los de pruebas. Distribución:
- `cmd`: 1 archivos.
- `internal`: 18 archivos.
- `tests`: 15 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
