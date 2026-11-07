# Requisitos no funcionales

Condiciones que el sistema debe cumplir, más allá de sus funciones. Los
identificadores `RNF-xx` son los del documento de requerimientos v2.5 y **no se
renumeran**. Cada requisito tiene una sola condición; la estrategia indica cómo
se cumple hoy.

## Scraping y actualización

| ID | Requisito | Condición | Cómo se cumple hoy |
| --- | --- | --- | --- |
| RNF-01 | Extracción concurrente | La extracción no bloquea las demás operaciones | Scraper Go como worker separado, con trabajos por Redis Streams |
| RNF-02 | Integridad de la información | Si la tienda se cae a mitad de la extracción, no se pierden los precios anteriores | Ingesta transaccional e idempotente en Catálogo ([D07](../Decisiones/07_Ingesta_idempotente.md)) |
| RNF-03 | Frecuencia de actualización | Las actualizaciones masivas corren en horarios de baja demanda, sin detener la plataforma | **Parcial:** los trabajos corren en segundo plano, pero no hay programación horaria (no existe un CronJob); hoy se lanzan a mano con el bot |
| RNF-04 | Incorporación de fuentes | Agregar un supermercado no afecta a los ya integrados | Un spider por cadena en `scraper_go/internal/spiders/` |

## IA y servicios externos

| ID | Requisito | Condición | Cómo se cumple hoy |
| --- | --- | --- | --- |
| RNF-05 | Límite de acción de la IA | La IA nunca modifica, infiere ni altera precios ni disponibilidad | Patrón anti-alucinación de las [reglas de negocio](Reglas_de_Negocio.md). *Cambio:* la versión original limitaba la IA a normalizar nombres; hoy también extrae filtros y redacta recetas, siempre sobre datos validados |
| RNF-06 | Resiliencia de servicios externos | Si la IA o un servicio externo rechaza peticiones, el sistema se recupera sin caerse | Parcial: respaldo entre Gemini y Groq (`fallback_proveedor.py`); los reintentos acotados y los casos de fallo van en SUP-315 |

## Mapas y ubicación

| ID | Requisito | Condición | Cómo se cumple hoy |
| --- | --- | --- | --- |
| RNF-07 | Cálculo realista del desplazamiento | El costo se calcula por calles reales y según el medio de transporte | **Diferido:** en el Sprint 3 la distancia se estima con Haversine y un factor declarado, y la interfaz lo rotula como estimado (SUP-323, [D11](../Decisiones/11_Motor_de_rutas_en_Go.md)) |
| RNF-08 | Privacidad geográfica | La ubicación exacta del usuario no se guarda de forma persistente después del cálculo | Sprint 3: las coordenadas viajan en la solicitud y no se guardan ni se registran en logs (SUP-316, SUP-338) |

## Visualización

| ID | Requisito | Condición | Cómo se cumple hoy |
| --- | --- | --- | --- |
| RNF-09 | Agrupación visual de precios | El mismo producto de distintas tiendas se muestra en una sola vista comparativa | Equivalencia por EAN (SUP-244); la vista agrupada queda pendiente (SUP-254) |
| RNF-10 | Retroalimentación visual | Los cálculos largos muestran un indicador de carga | Estados de carga en las pantallas del recorrido (SUP-330, SUP-261) |

## Seguridad y comunicaciones

| ID | Requisito | Condición | Cómo se cumple hoy |
| --- | --- | --- | --- |
| RNF-11 | Protección de comunicaciones | Toda comunicación entre cliente y servidor va cifrada | **Sin verificar:** los manifiestos de Ingress no declaran TLS. Confirmar si el clúster del curso entrega HTTPS por su cuenta |
| RNF-12 | Protección de acceso | Los puntos críticos resisten la automatización y la fuerza bruta | Rate limit en Redis para login, registro y Google; contraseñas con bcrypt ([D08](../Decisiones/08_Rate_limit_falla_cerrado.md)) |

## Restricciones del entorno

- **Todo corre en el clúster de Kubernetes del curso**, con cuotas estrictas de
  memoria y CPU por namespace ([escalado y recursos](../Metricas/02_Escalado_y_recursos.md)).
  Una fuga de memoria o una consulta sin paginar puede matar un pod: toda
  consulta de listas se pagina y las imágenes parten de bases `-alpine`, `-slim`
  o `distroless`.
- **El desarrollo local usa Docker Compose** con límites de memoria parecidos,
  para que lo que funciona en local funcione en el clúster.
- **La IA se usa como servicio externo** (Gemini o Groq); no hay modelos locales.
- **El sistema debe levantarse con un solo comando** (`docker compose up`).

## Criterios de calidad

- **Usabilidad:** entender la comparación de precios y si el desplazamiento
  compensa el ahorro.
- **Rendimiento:** respuesta ágil en las búsquedas más frecuentes.
- **Seguridad:** autenticación protegida y comunicaciones cifradas.
