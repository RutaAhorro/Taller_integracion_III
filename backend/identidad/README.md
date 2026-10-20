# Identidad

Servicio Go independiente, propietario de `api.usuarios`. Expone las rutas
públicas `/api/v1/auth/*` y, para administración, `GET/PUT
/api/v1/admin/usuarios` con `RequireRole('admin')`; Gateway conserva ambas
como entrada pública. No importa API, Catálogo ni coordinación de scraping, y
no ejecuta migraciones: requiere `06_propiedad_permisos.sql` y, desde SUP-242,
`08_normalizar_rol_admin_y_auditoria.sql` (auditoría y normalización del rol
`administrador` a `admin`). Las rutas protegidas revalidan estado y rol en
`api.usuarios` en cada petición.

```bash
go test -race ./...
go vet ./...
go build ./cmd/identidad
```

Configuración, seguridad, contratos y operación:
[GUIA_IDENTIDAD_DESACOPLADA.md](../../docs/Guias/GUIA_IDENTIDAD_DESACOPLADA.md).
Integración aislada desde la raíz: `bash scripts/smoke_identity_compose.sh`.

Correo SMTP (SUP-189): `identidad correo-prueba <destinatario>` envía un correo
real con la configuración `SMTP_*` del entorno; ver la sección de correo de la guía.
