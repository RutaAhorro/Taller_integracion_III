# Arquitectura — Seguridad transversal

[← Índice de arquitectura](README.md)

| Mecanismo | Dónde | Detalle |
|---|---|---|
| Sesión | Identidad | JWT HS256 en la cookie HttpOnly `jwt`: 24 h, `SameSite=Lax`, `Secure` según el entorno |
| Cierre de sesión | Identidad | `POST /auth/logout` expira la cookie (#124) |
| Autorización | Cada servicio | El Gateway no valida JWT; las rutas protegidas revalidan estado y rol |
| Anti-suplantación | Gateway | Elimina las cabeceras `X-User-ID` y `X-Role` que envía el cliente |
| Rate limit | Identidad + Redis | Por IP cada 15 min: 5 logins fallidos, 30 registros y 30 logins Google. Sin Redis, falla cerrado |
| Tokens de servicio | Control, Catálogo | `SCRAPING_SERVICE_TOKEN` y `CATALOGO_INTERNAL_TOKEN`, comparados en tiempo constante. Nunca llegan al navegador |
| Recuperación de contraseña | Identidad | Cola asíncrona, para que el tiempo de respuesta no revele si la cuenta existe. Los tokens se guardan como hash |
| Google SSO | Identidad | Validación criptográfica y protección contra *pre-account hijacking* al vincular cuentas |
| Contraseñas | Identidad | bcrypt, con su límite de 72 bytes. Cambiar correo o contraseña exige la contraseña actual |
| Variables públicas | Frontend | Las variables `VITE_*` son visibles en el navegador, así que no llevan secretos |

## Límites conocidos

- No hay lista negra de JWT: un token copiado sigue siendo válido hasta su `exp`, aunque se haga logout.
- Sin Redis, el rate limiter responde 500; lo correcto sería 503.
- No existe un OTP operativo.
