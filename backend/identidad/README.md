# Identidad

Servicio Go independiente, propietario de `api.usuarios`. Expone las cinco
rutas existentes `/api/v1/auth/*`; Gateway las conserva como entrada pública.
No importa API, Catálogo ni coordinación de scraping, y no ejecuta migraciones.

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
