# SUP-266: módulos de Identidad y coordinación de scraping

> Estado posterior: SUP-267 extrajo Catálogo y SUP-268 retira su lector de la API.
> `main.go` compone ahora únicamente Identidad y Scraping. Los paquetes antiguos
> `domain`, `handlers`, `repositories`, `services` y `routes` de productos se
> eliminan; sus pruebas de negocio permanecen en Catálogo. La persistencia de
> ingesta y los modelos heredados continúan hasta SUP-269/SUP-271.
> Ver [procedimiento vigente](GUIA_CATALOGO_ENRUTAMIENTO.md).

## Alcance y resultado

Identidad y Scraping tienen implementaciones privadas y entradas pequeñas para
registrar rutas. Continúan ejecutándose en el mismo binario de `backend/api`,
detrás de la Gateway creada en SUP-264 e integrada en SUP-265. Catálogo conserva
por ahora sus paquetes actuales; su extracción corresponde a SUP-267.

Este cambio separa código y contratos de persistencia. El escalado independiente
de Catálogo llegará con su extracción y despliegue en SUP-267/SUP-268.

## Estructura

```text
backend/api/
├── main.go                      # Composición de los módulos y de Catálogo
├── internal/
│   ├── identity/
│   │   ├── module.go             # New, RegisterRoutes, RequireAuth, RequireRole
│   │   ├── internal/
│   │   │   ├── domain/           # Usuario, DTO y contrato UsuarioRepository
│   │   │   ├── services/         # Registro, login, Google y actualización
│   │   │   ├── repositories/     # Adaptador GORM de usuarios
│   │   │   ├── handlers/
│   │   │   ├── routes/
│   │   │   ├── middleware/       # JWT, permisos y límite de login
│   │   │   └── security/         # Firma/verificación JWT y bcrypt
│   │   └── tests/
│   └── scraping/
│       ├── module.go            # New y RegisterRoutes
│       ├── internal/
│       │   ├── domain/          # DTO, TrabajoRepository e IngestaRepository
│       │   ├── services/        # Ciclo de trabajos y validación de ingesta
│       │   ├── repositories/
│       │   │   ├── trabajo_repository.go
│       │   │   └── ingesta_legacy_repository.go
│       │   ├── handlers/        # HTTP y despacho a Redis
│       │   └── routes/
│       └── tests/
├── domain/, handlers/, repositories/, services/, routes/
│                                # Catálogo pendiente de SUP-267
├── middleware/error_handler.go  # Respuestas HTTP y recuperación de errores
├── utils/security.go            # Sanitización genérica de texto
└── infrastructure/models.go     # Modelos heredados pendientes de partición
```

Go impide importar el `internal` anidado de un módulo desde otro módulo o desde
Catálogo. `main.go` accede a las entradas públicas y entrega los interceptores de
Identidad a las rutas de administración de Catálogo. Catálogo recibe funciones
HTTP y no importa JWT, repositorios de usuarios ni entidades de Identidad.

Los servicios dependen de interfaces de su propio dominio, no de repositorios
concretos. No existe una biblioteca compartida de entidades entre los módulos.
`tests/architecture_test.go` verifica estas restricciones y permite el acceso a
los modelos heredados únicamente al adaptador transitorio de ingesta.

## Datos y compatibilidad

Se conservan las rutas `/api/v1/auth/*`, `/api/v1/productos*`,
`/api/v1/admin/productos` y `/api/v1/scraper/*`, junto con sus códigos HTTP,
formatos JSON, cookies y documentación Swagger publicada.

Identidad conserva su comportamiento de registro, Google, perfil, JWT y rate
limiting. Scraping conserva estados de trabajos, validaciones, ingesta directa o
con trabajo y `LPUSH scraper:jobs` con el formato consumido por el worker. El
worker sigue enviando resultados por HTTP.

No hay migraciones SQL, cambios de claves foráneas, nuevas credenciales de base
de datos ni alteraciones de permisos. Los modelos privados mantienen los nombres
de tabla y los mapeos GORM existentes. `disparado_por_usuario_id` sigue siendo una
referencia UUID con su restricción actual en PostgreSQL.

Los modelos de usuario, cadena, sucursal y trabajo que siguen en
`infrastructure/models.go` son representaciones heredadas; no se usan en los
casos de uso de Identidad ni en la persistencia de trabajos del nuevo módulo.
Las relaciones de otros modelos y el sembrador de productos conservan sus
representaciones actuales. Esta coexistencia es transitoria y requiere revisar
sus consumidores antes de eliminar modelos durante la partición de datos.

`ingesta_legacy_repository.go` mantiene la escritura de productos crudos,
capturas y contador del trabajo en la transacción SQL existente. Separar las
interfaces no elimina ese acoplamiento de datos ni añade idempotencia o
recuperación de entregas. SUP-269 trasladará la escritura de productos/precios a
Catálogo; SUP-270 resolverá recuperación y entregas; SUP-271 consolidará propiedad
de tablas y permisos. No debe añadirse otro acceso a modelos heredados desde los
servicios privados.

## Validación y criterio de salida

Desde `backend/api`:

```bash
go test -race ./...
go vet ./...
go build -o /tmp/api-sup266 .
```

Desde la raíz del repositorio, con Docker y Python 3:

```bash
bash scripts/smoke_gateway_compose.sh
```

El humo utiliza una base, red y volumen aislados. Cubre registro/login/cookies,
actualización de perfil sin escalada de rol, rechazos de administración, consulta
de productos, trabajos, despacho a Redis, ingesta y finalización. No arranca el
worker ni ejecuta scraping real. También comprueba frontend → Gateway → API y
el retorno a entrada directa descrito en la guía de SUP-265.

Las pruebas de cada dominio se trasladaron con su implementación. La prueba de
composición utiliza el router real y verifica autenticación, permisos, respuestas
de error y el comportamiento sin Redis. El workflow `gateway-checks.yml` añade
pruebas con detector de carreras, límites de importación, vet y compilación de
la API a los controles existentes de Gateway e integración.

La tarea está lista para integrar cuando estos controles pasan y los contratos
de frontend, bot y worker conservan su comportamiento. Regenerar Swagger tras
cambios posteriores requiere incluir paquetes internos (`swag init --parseInternal`)
y revisar que el contrato publicado siga siendo compatible.

## Despliegue y rollback

Se utiliza la imagen y configuración actuales de la API. No se crean servicios,
Deployments ni HPA en esta etapa. La Gateway continúa reenviando a `api-backend`.

Para revertir un despliegue de SUP-266, se restaura la imagen de API previa a esta
tarea y se verifica el humo. La base de datos permanece compatible porque esta
etapa no modifica el esquema. El rollback de la Gateway es un procedimiento
distinto, descrito en `GUIA_GATEWAY_DESARROLLO_PRUEBAS.md`.

## Siguiente tarea

SUP-267 extraerá Catálogo y precios como servicio con módulo Go y binario propios.
Estas fronteras permiten hacerlo sin arrastrar registro, JWT ni coordinación de
trabajos al nuevo servicio. Su activación en la Gateway y los despliegues se
realizarán en SUP-268.

La implementación y comparación independiente de SUP-267 se documentan en
[Catálogo independiente](GUIA_CATALOGO_INDEPENDIENTE.md). El lector anterior
permanece en la API hasta activar el nuevo destino en SUP-268.

## Comprobaciones realizadas en esta rama

- 69 comprobaciones existentes conservadas y 20 nuevas de composición y
  dependencias: 89 resultados aprobados, incluyendo subpruebas, con `go test -race`.
- `go vet`, compilación de API, construcción de su imagen Docker y validación
  del workflow con `actionlint`: aprobados.
- Humo con la imagen refactorizada: 27 comprobaciones API/Gateway y 10 de retorno
  a entrada directa, más frontend servido y enrutado por Nginx: aprobados.
- Swagger regenerado en directorios temporales desde la base y desde esta rama:
  documentos idénticos, con 16 rutas y 22 esquemas. Los archivos publicados se
  conservan.
- Campos, etiquetas y tablas de los modelos privados, y las seis operaciones SQL
  de scraping, conservados respecto de la base.

La validación de integración se ejecutó con recursos aislados locales. No se ha
desplegado esta tarea en el clúster de la universidad ni ejecutado su nuevo job
de CI en GitHub.
