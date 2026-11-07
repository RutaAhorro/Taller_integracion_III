# D09 — Logout del lado del servidor

[← Índice de decisiones](README.md)

**Contexto:** la cookie `jwt` es HttpOnly, así que el frontend no puede borrarla. Antes de #124, al "cerrar sesión" solo se limpiaba el estado de la UI y la cookie seguía siendo válida por 24 h. En un equipo compartido, el siguiente usuario podía entrar a la cuenta.

**Decisión:** crear `POST /api/v1/auth/logout` en Identidad, que funciona así:
- Expira la cookie con los mismos atributos con que se emitió.
- Responde `204`.
- Es idempotente: responde igual aunque no haya sesión.

**Motivo:** es la única forma de eliminar una cookie HttpOnly. Además, el cambio es mínimo: no requiere migraciones ni variables nuevas.

**Alternativa descartada:** una lista negra de JWT en Redis. Permitiría revocar tokens que ya fueron copiados, pero agrega estado y una consulta extra en cada petición autenticada. Queda fuera de alcance.

**Consecuencias:**
- Un JWT que alguien copió antes del logout sigue siendo válido hasta su `exp`.
- Pendiente: el `logout()` del frontend todavía no llama a este endpoint.
