# D06 — Redis Streams con outbox

[← Índice de decisiones](README.md)

**Contexto:** Control debe hacer dos cosas al crear un trabajo: guardarlo en la base de datos y avisar al worker. Si se guarda y la publicación falla, el trabajo queda huérfano. Si se publica y la transacción falla, el worker procesa un trabajo que no existe.

**Decisión:**
- Control escribe el trabajo y un registro de **outbox** en la misma transacción.
- Un publicador lee el outbox y lo lleva a **Redis Streams**.
- El worker consume desde el stream.

**Motivo:**
- El outbox hace que la publicación sea recuperable: lo que no se publicó se reintenta.
- Redis Streams conserva los mensajes y permite grupos de consumidores con confirmación (ack).
- Redis ya era parte del stack, así que no se agrega infraestructura.

**Alternativa descartada:**
- **RabbitMQ o Kafka:** son más robustos, pero agregan un servicio pesado a un clúster limitado.
- **Pub/Sub de Redis:** no conserva los mensajes, así que si el worker está caído, el trabajo se pierde.
- **Publicar directamente sin outbox:** expone la inconsistencia descrita en el contexto.

**Consecuencias:**
- Se garantiza la entrega *al menos una vez*, así que un mismo lote puede llegar repetido. Por eso la ingesta es idempotente ([D07](07_Ingesta_idempotente.md)).
- El publicador del outbox es una pieza más que mantener.
