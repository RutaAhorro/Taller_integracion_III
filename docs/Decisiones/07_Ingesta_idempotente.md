# D07 — Ingesta idempotente por lotes

[← Índice de decisiones](README.md)

**Contexto:** los reintentos del worker, de Control y del outbox pueden hacer que un mismo lote de productos llegue más de una vez a Catálogo.

**Decisión:**
- Catálogo expone un endpoint interno de ingesta, protegido con `CATALOGO_INTERNAL_TOKEN` y sin ruta pública en el Gateway.
- Catálogo confirma productos, precios y un **recibo de lote** en una sola transacción.
- Si el lote ya tiene recibo, no se vuelve a aplicar.
- Control registra las entregas de forma durable antes de reenviarlas.

**Motivo:** con la idempotencia, reintentar siempre es seguro. Así se pueden usar reintentos agresivos sin riesgo de duplicar precios ni de inflar el historial.

**Alternativa descartada:** la entrega *exactamente una vez*. Es muy costosa de garantizar en sistemas distribuidos y no hace falta si el receptor es idempotente.

**Consecuencias:**
- Cada lote necesita un identificador estable.
- La tabla de recibos crece con el tiempo y eventualmente habrá que depurarla.
