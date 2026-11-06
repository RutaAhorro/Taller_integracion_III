# Arquitectura — Redes y despliegue

[← Índice de arquitectura](README.md)

## Docker Compose (local)

### Redes

- `microservices_net`: red común de todos los servicios.
- `identidad_edge`: el Gateway tiene la IP fija `172.30.0.2` e Identidad el alias `identidad-upstream`. Así Identidad solo confía en ese proxy para leer la IP real del cliente, que necesita para el rate limit.

### Puertos publicados

| Servicio | Host |
|---|---|
| Frontend | `3000` |
| Gateway | `127.0.0.1:8082` |
| PostgreSQL | `5432` |
| Redis | `6379` |
| Bot | `8081` |
| IA | `127.0.0.1:8002` |

### Límites de memoria

Los límites de Compose y los requests y limits de Kubernetes están en
[escalado y recursos](../Metricas/02_Escalado_y_recursos.md).

### Perfiles opcionales

- `rutas`: levanta el motor de rutas.

## Kubernetes (clúster del curso)

| Host | Ruta → destino |
|---|---|
| `api-vmatus.dev.censei.cl` | `/` → Gateway |
| `miweb-vmatus.dev.censei.cl` | `/api/v1` → Gateway; `/` → Frontend |

Varios servicios tienen manifiestos HPA. El inventario del Sprint 2 lo informó el equipo; no es una consulta en vivo del clúster.

## Desarrollo local

El hot reload usa un `docker-compose.override.yml` local, sin versionar. Ese archivo cambia el frontend al target `development` y le da 768M de memoria.
