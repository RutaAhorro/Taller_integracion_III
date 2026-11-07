# D03 — Gateway que solo enruta

[← Índice de decisiones](README.md)

**Contexto:** el Gateway es el punto de entrada de toda la API. Es tentador centralizar ahí la validación del JWT, los límites de tasa o algunas reglas de negocio.

**Decisión:** el Gateway (Go `net/http`) solo enruta por prefijo. No valida el JWT, no ejecuta SQL, no usa Redis y no aplica reglas de negocio. Sus únicas tareas transversales son tres:
- Generar el `X-Request-ID`.
- Eliminar las cabeceras `X-User-ID` y `X-Role` que vengan del cliente.
- Usar transportes separados para lecturas y escrituras.

**Motivo:**
- Si el Gateway decidiera la autorización, los servicios tendrían que confiar en cabeceras que puede forjar cualquiera que llegue a la red interna.
- Con la autorización en cada servicio, no hace falta confiar en la red.
- El Gateway queda pequeño, sin estado y fácil de probar.

**Alternativa descartada:** validar el JWT en el Gateway y propagar la identidad por cabeceras. Duplica la lógica de sesión y hace que el aislamiento dependa de que nadie se salte el Gateway.

**Consecuencias:**
- Cada servicio valida el JWT por su cuenta (HS256 y `exp` obligatorio), lo que duplica algo de código.
- Todos los servicios comparten el `JWT_SECRET`.
