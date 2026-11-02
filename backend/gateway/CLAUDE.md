# Gateway — contexto IA

- Leer `README.md`. Go estándar `net/http`, entrada `cmd/gateway/main.go`.
  Puerto configurado 8080 en Compose; predeterminado del binario 8082.
- Proxy en `internal/proxy/gateway.go`, configuración en `internal/config`.
- Sin SQL, Redis, JWT_SECRET ni lógica de negocio. No agregar autenticación
  centralizada ni agregación de métricas; cada servicio aplica sus permisos.
- Enrutar auth/admin usuarios a Identidad, productos/admin productos a Catálogo,
  scraper/health a Control; listas solo con RUTAS_URL. Chat/rutas/misiones no
  están enrutados. Prefijos delimitados, sin fallback entre dominios.
- Mantener cookies, path/query codificados, Authorization y X-Request-ID.
  Eliminar headers de identidad falsificables; forwarding solo por proxies fiables.
- No reintentar escrituras automáticamente. Live/ready locales; diagnóstico
  de dependencias separado para no retirar rutas sanas por otro dominio caído.
- Pruebas desde este servicio: `go test -race ./...`, `go vet ./...`.

<!-- ARCHITECTURE:AUTO-GENERATED — NO EDITAR DEBAJO DE ESTA LÍNEA -->

## Índice breve generado

10 archivos fuente, incluidos los de pruebas. Distribución:
- `cmd`: 1 archivos.
- `internal`: 9 archivos.

Buscar símbolos en el código; este índice no acredita funciones desplegadas.
