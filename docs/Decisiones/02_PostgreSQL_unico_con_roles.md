# D02 — Una instancia PostgreSQL con roles por dominio

[← Índice de decisiones](README.md)

**Contexto:** el patrón de microservicios sugiere una base de datos por servicio. El clúster del curso tiene recursos limitados: la base de datos tiene un límite de 300M.

**Decisión:** usar una sola instancia PostgreSQL/PostGIS, con un rol por dominio (`app_identity`, `app_catalogo`, `app_scraping`, `app_rutas`) y permisos por tabla (migraciones 05–09). Cada servicio solo puede tocar sus propias tablas, y está prohibido el SQL entre dominios.

**Motivo:**
- Tiene el costo de operar una sola base: un volumen, un respaldo y un proceso.
- Aun así, la propiedad de cada tabla la impone el motor de base de datos, no una convención del equipo.

**Alternativa descartada:** una instancia por servicio. Da un aislamiento físico real, pero multiplica la memoria, los volúmenes y la operación, algo que el clúster del curso no soporta bien.

**Consecuencias:**
- Si cae la instancia, caen todos los dominios (punto único de falla).
- Los esquemas tienen nombres históricos (`api`, `scraper`) que no corresponden a los servicios actuales; por ejemplo, `scraper` contiene tablas de Catálogo y de Control.
- En volúmenes existentes, las migraciones de permisos se aplican a mano.
