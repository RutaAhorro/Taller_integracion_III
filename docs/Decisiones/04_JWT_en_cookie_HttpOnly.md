# D04 — JWT en cookie HttpOnly

[← Índice de decisiones](README.md)

**Contexto:** la sesión del frontend React necesita un token que el navegador envíe en cada petición.

**Decisión:** Identidad emite un JWT HS256 en la cookie `jwt`, con estos atributos:
- `HttpOnly`.
- `SameSite=Lax`.
- `Secure` según el entorno.
- Validez de 24 h.

El frontend la envía con `credentials: 'include'`. El usuario que se guarda en localStorage es solo para la UI y no acredita la sesión.

**Motivo:** un script inyectado (XSS) no puede leer una cookie HttpOnly. Con `SameSite=Lax`, el navegador no envía la cookie en las peticiones POST que se originan en otros sitios, lo que mitiga el CSRF.

**Alternativa descartada:** guardar el JWT en localStorage y enviarlo en un header `Authorization`. Es más simple, pero cualquier XSS podría robar el token.

**Consecuencias:**
- El frontend no puede borrar la cookie, así que hace falta un logout en el servidor ([D09](09_Logout_en_servidor.md)).
- El token no se puede revocar antes de que expire: no hay lista negra.
- El CORS se configura con orígenes exactos y credenciales.
