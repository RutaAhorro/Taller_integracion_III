# D08 — Rate limit en Redis que falla cerrado

[← Índice de decisiones](README.md)

**Contexto:** los endpoints de login, registro y Google están expuestos a ataques de fuerza bruta y a creación masiva de cuentas.

**Decisión:**
- Identidad aplica contadores por IP en Redis, con scripts Lua atómicos.
- Los límites, por IP cada 15 minutos, son: 5 logins fallidos, 30 registros y 30 logins con Google.
- Si Redis no responde, la petición se **rechaza** (falla cerrado).
- La IP real se lee solo cuando viene del proxy de confianza, que es el Gateway en la red `identidad_edge`.

**Motivo:**
- Los scripts Lua evitan condiciones de carrera entre leer y escribir el contador.
- Fallar cerrado impide que un atacante desactive la protección tumbando Redis.
- La red con una IP fija para el Gateway impide falsificar `X-Forwarded-For`.

**Alternativa descartada:**
- **Contadores en memoria de cada réplica:** no funcionan bien con el HPA, porque cada réplica tiene su propio contador.
- **Fallar abierto:** prioriza la disponibilidad sobre la seguridad en el endpoint más sensible.

**Consecuencias:**
- Si Redis se cae, nadie puede iniciar sesión.
- Hoy el rechazo responde `500`; lo semánticamente correcto sería `503`.
- Varios usuarios detrás de un mismo NAT comparten el contador.
