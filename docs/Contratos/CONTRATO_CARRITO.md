# Carrito persistente por cuenta

Implementación en esta rama; requiere migración y despliegue autorizados.
No acredita que el flujo esté publicado.

## Propiedad y persistencia

Motor/Listas guarda un borrador por cuenta en `rutas.carritos` (migración 14).
No es una lista confirmada ni consume el límite de listas del usuario. Guarda
UUID de producto y envases: no precios, stock, credenciales ni referencias SQL
de otros dominios. Los límites son los mismos que los artículos: 100 productos
distintos, entre 1 y 99 envases por producto; no admite repetidos.

La cuenta sale de la cookie HttpOnly `jwt`. Todas estas rutas exigen además
`X-Carrito-Cuenta` igual al `user_id` autenticado. Esa cabecera no autentica:
sólo evita que una pestaña antigua escriba bajo una cookie cambiada por otra
cuenta. Se rechaza con 401 si falta o no coincide. No se recibe dueño en el JSON.
Gateway ya enruta el prefijo `/api/v1/listas`; no añade autorización ni SQL.

## API

| Método | Ruta bajo `/api/v1/listas/carrito` | Cuerpo |
| --- | --- | --- |
| GET | raíz | — |
| PUT | raíz | `{"version":0,"articulos":[{"producto_id":"UUID","cantidad":2}]}` |
| POST | `/preparar` | `{"version":1,"lista_id":"UUID","clave":"UUID de operación"}` |
| POST | `/confirmar` | `{"clave":"misma clave"}` |

Cada éxito devuelve 200 y el documento:

```json
{
  "version": 2,
  "articulos": [{"producto_id": "UUID", "cantidad": 2}],
  "envio": {"lista_id": "UUID", "clave": "clave-opaca"},
  "ultimo_envio": null,
  "disponibilidad": {}
}
```

La versión inicial es 0. `envio` y `ultimo_envio` pueden ser null.
`disponibilidad` se consulta en Catálogo por HTTP, por UUID, con
`estado_producto` y `producto`; si no responde queda vacía y no se inventan
precios ni stock. No se guarda en PostgreSQL. Respuestas privadas: `no-store`.

### Edición y concurrencia

PUT sustituye el borrador sólo si su versión coincide y no hay envío pendiente.
Si el contenido ya es exactamente el guardado y no hay envío, repetirlo es
inocuo y no incrementa la versión, incluso si se perdió la respuesta anterior.
Un cambio real incrementa la versión. 409 obliga a recuperar y reconciliar;
no se sobrescribe automáticamente otra pestaña/dispositivo.

### Preparación y confirmación

Preparar exige una lista del usuario y un carrito no vacío. Congela los
artículos, la clave y el destino antes de modificar la lista. Una preparación
repetida con el mismo destino y clave no genera otro envío.

Confirmar no acepta artículos del cliente: toma el lote congelado. En una sola
transacción bloquea carrito y lista, valida existencia por HTTP a Catálogo,
comparte el ledger y las reglas del lote, aplica los artículos, guarda el recibo
y vacía el borrador. Reintentar la misma clave no suma dos veces. Un recibo
antiguo nunca vacía productos de una compra posterior.

El recibo `ultimo_envio` incluye `lista_id`, `clave` y `aplicado`. Un rechazo de
negocio devuelve **200 con `aplicado:false`**, conserva productos y libera la
edición. Incluye `motivo` (`max_articulos`, `idempotency_conflict`,
`lista_no_existe`, `productos_rechazados`) y, si corresponde, `rechazados` con
UUID y motivo (`no_encontrado`, `cantidad_maxima`). **200 no basta para mostrar
éxito: el frontend comprueba `aplicado`.** Sin stock o desconocido se admite con
advertencia, como el contrato de artículos.

Errores de SQL/Catálogo no confirman nada: rollback y preparación intacta.
Si COMMIT ocurrió pero se perdió la respuesta, GET permite recuperar el recibo.
La tabla conserva el último recibo, no un historial ilimitado de confirmaciones.
Claves más antiguas o una versión incompatible se rechazan sin modificar datos.

Errores: 400 (JSON/campos/límites), 401 (sesión/cuenta incoherente), 404
(destino ajeno o inexistente al preparar), 409 (versión/envío), 503 (Catálogo
no disponible), 500 (persistencia inesperada). Envoltorio habitual de Motor.

## Recuperación del navegador

`rutaahorro.carrito.v1.{usuarioID}` contiene borrador, versión y preparación
pendiente, no secretos. Los cambios se respaldan antes de enviar; luego se
sincronizan secuencialmente. Se recuperan al recargar, reconectar o volver a la
ventana. Una copia ilegible se respalda como `.recuperacion` si Storage lo permite.

Un borrador local pendiente sólo se reenvía sobre su versión base. Si el remoto
cambió, se muestran ambos: adoptar el servidor descarta explícitamente el local,
y conservar el local reemplaza explícitamente el remoto, con una nueva
comprobación de versión. No se reemplaza un envío preparado.

La copia local no garantiza durabilidad si el navegador deniega/limpia Storage.
La UI distingue pendientes de confirmados; sólo lo confirmado por Motor se
recupera desde otro dispositivo. Al recuperar se invalidan precios locales
hasta consultar Catálogo. Cambiar de cuenta desmonta el coordinador y no
reenvía su cola con una cookie posterior.

## Orden de habilitación en Kubernetes

1. Revisar y respaldar la BD; con autorización aplicar `14_carritos_persistentes.sql`
   como operador, sin recrear volúmenes ni ejecutar scripts de credenciales.
2. Verificar SELECT/INSERT/UPDATE de `app_rutas` sobre la nueva tabla. No se
   requiere DELETE ni acceso a tablas de Identidad/Catálogo.
3. Publicar Motor y después frontend con imágenes revisadas. No cambiar
   variables compartidas, Services ni cuotas por esta funcionalidad.
4. Verificar aislamiento entre cuentas, recargas, otros dispositivos, offline,
   conflictos y respuesta perdida. La guía está en el README frontend.

No se ejecutan migraciones al arrancar. Si falta 14, la nueva imagen de Motor
no abre HTTP. Ante rollback de imágenes se conserva la tabla y los borradores;
no hay un borrado automático de carritos ni migración destructiva de listas.
