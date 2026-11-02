# Docker para desarrollo de RutaAhorro

Esta guía complementa el [arranque del README principal](../../README.md), que
contiene las variables obligatorias y el inventario actual. No es una receta de
despliegue en Kubernetes.

## Requisitos

Docker Engine con Compose v2 en Linux, o Docker Desktop con integración WSL 2 en
Windows. Seguir las instrucciones oficiales de instalación para la distribución
y versión del sistema; no mezclar repositorios de Debian con Ubuntu.

```bash
docker --version
docker compose version
```

En Linux, si Docker usa systemd: `sudo systemctl start docker`. En Windows,
iniciar Docker Desktop. Si hay un error de permisos del socket, consultar al
operador: pertenecer al grupo `docker` otorga privilegios equivalentes a root.
No resolver el problema haciendo público el socket o con permisos `777`.

## Configurar y arrancar

Desde la raíz, preparar `.env` a partir de `.env.example` sin sobrescribir un
archivo existente. Completar contraseñas y tokens diferentes antes de ejecutar:

```bash
docker compose config --quiet
docker compose up -d --build db redis identidad catalogo control_scraping gateway frontend
docker compose ps
```

Aplicación: <http://localhost:3000>. Gateway: <http://127.0.0.1:8082>.
No hace falta compilar mapas ni levantar OTP. El scraper actual es Go; el bot,
la IA y el motor de listas requieren su configuración específica. Consultar el
README principal antes de levantar todos los servicios con un `up` sin selección.

## Comandos cotidianos

```bash
docker compose logs -f gateway identidad catalogo control_scraping
docker compose logs -f scraper_worker
docker compose up -d --build frontend
docker compose down
```

- Para recarga automática del frontend, usar el target `development` según
  [su README](../../frontend/web/README.md). El target predeterminado es Nginx.
- Un contenedor `Running` no acredita un flujo funcional; revisar readiness,
  logs y una solicitud real. `depends_on` organiza el arranque, no garantiza
  disponibilidad permanente.
- No mostrar `docker compose config` completo al compartir diagnósticos: expone
  valores interpolados. Usar `--quiet` para validar sin imprimirlos.

## Persistencia y seguridad

Los volúmenes `pgdata`, `redisdata` y `scraperdata` contienen datos y recuperación.
No ejecutar `down -v` ni borrar volúmenes para resolver un error de permisos o
aplicar una migración. Los scripts SQL iniciales solo corren sobre PostgreSQL
vacío; ver [migraciones](../../infrastructure/db/README.md) para una base existente.

No publicar `.env`, imágenes con secretos, spools o respuestas con datos personales.
Los puertos PostgreSQL/Redis expuestos por Compose son de desarrollo; no exponer
ese entorno a Internet. Ajustar CORS, cookies Secure y proxies al entorno real.
