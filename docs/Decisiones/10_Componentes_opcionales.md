# D10 — Componentes opcionales por perfil u opt-in

[← Índice de decisiones](README.md)

**Contexto:** el motor de rutas y la IA conversacional están incompletos o no están integrados con el frontend. Si arrancaran siempre, consumirían memoria y podrían fallar por falta de configuración (base de datos de listas, claves de Gemini o Groq).

**Decisión:**
- **Motor de rutas:** usa el perfil de Compose `rutas`, y el Gateway solo enruta `/api/v1/listas*` si existe `RUTAS_URL`.
- **IA, bot y worker:** se levantan aparte del conjunto base.
- **Vistas del frontend:** las de misiones, rutas e historial se marcan como simuladas.

**Motivo:** el conjunto base (db, redis, identidad, catálogo, control, gateway, frontend) arranca sin configuración adicional. Además, el sistema no promete funciones que aún no existen.

**Alternativa descartada:** levantar siempre todos los servicios. Fallarían o consumirían memoria sin aportar nada, y harían parecer que las funciones incompletas ya están integradas.

**Consecuencias:**
- El enrutamiento depende de la configuración, así que hay que recordar definir `RUTAS_URL` en cada entorno.
- La IA sigue sin ruta en el Gateway y el frontend llama a `/chat/gemini`, que no existe (SUP-253).
