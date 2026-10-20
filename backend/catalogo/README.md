# Catálogo y precios

Servicio Go independiente para consultar productos, precios, historial y catálogo
administrativo, y persistir lotes enviados por coordinación de scraping mediante
una API interna autenticada. No requiere Redis ni importa paquetes de `backend/api`.

```bash
cd backend/catalogo
cp .env.example .env
# Completar DB_URL y JWT_SECRET en .env antes de ejecutar.
set -a
source .env
set +a
go run ./cmd/catalogo
```

El servicio lee variables de entorno; no carga `.env` automáticamente. La clave
JWT debe coincidir con la utilizada por Identidad. La configuración de ejemplo
usa el puerto 8083 para convivir con la API actual.

`CORS_ALLOWED_ORIGINS` contiene orígenes HTTP(S) exactos separados por comas
(sin rutas ni comodines). Configurar los orígenes del frontend para cada entorno;
el ejemplo habilita sólo dos puertos locales. Si está vacío, las peticiones con
`Origin` reciben 403; las peticiones sin `Origin` siguen disponibles y requieren
JWT/rol en administración. CORS no reemplaza autenticación ni protección CSRF.

`IDENTIDAD_URL` es el origen HTTP(S) interno de Identidad. Catálogo reenvía la
cookie `jwt` a `GET /api/v1/auth/me` antes de autorizar `/api/v1/admin/productos`;
así una cuenta desactivada o degradada pierde acceso sin esperar a que expire el
token. Si la variable está vacía, el catálogo público sigue funcionando, pero la
administración falla cerrada con 503. En Kubernetes, la NetworkPolicy de Identidad
debe permitir el ingreso desde el pod `catalogo`.

```bash
go test -race ./...
go vet ./...
go build -o /tmp/catalogo ./cmd/catalogo
```

Desde la raíz del repositorio:

```bash
docker build -t catalogo:local backend/catalogo
bash scripts/smoke_catalogo_compose.sh
bash scripts/smoke_catalogo_ingesta_compose.sh
```

Consulta la [guía de SUP-267](../../docs/Guias/GUIA_CATALOGO_INDEPENDIENTE.md)
para rutas, permisos, pruebas, límites de compatibilidad y activación en SUP-268.

La [guía de ingesta SUP-269](../../docs/Guias/GUIA_CATALOGO_INGESTA.md) describe
el token privado, la propiedad de escrituras, las ventanas de fallo entre
Catálogo y el contador del trabajo, el despliegue y el rollback. Con
`CATALOGO_INTERNAL_TOKEN` vacío, el servicio conserva el modo de sólo lectura.
