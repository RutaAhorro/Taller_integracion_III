# Contrato de artículos de lista

> **Estado:** borrador para revisión · SUP-321 · Responsable: Esban
> **Revisan:** Marcelo (Catálogo, SUP-312 y SUP-244), Renato (IA, SUP-325), Vicho (Rutas, SUP-316) y Daniela (frontend, SUP-252 y SUP-329)
> **Fecha límite:** 07-11-2026

## Para qué existe

Un producto pasa por cuatro servicios en el recorrido de la demo: Catálogo lo
publica, la IA lo recomienda, Listas lo guarda y Rutas calcula dónde comprarlo.
Si cada uno lo identifica distinto, todo funciona por separado y falla al
juntarlo. Este contrato fija **cómo se identifica un producto, cómo se guarda
en una lista y qué responde cada operación**.

Una vez cerrado, cambiarlo EXIGE avisar antes en el canal del equipo y abrir un
PR que apruebe quien consume la parte afectada (Rutas, IA, Catálogo o frontend).
No basta con avisar: el cambio se acuerda.

## 1. Cómo se identifica un producto

**Un artículo guarda el `producto_id` de Catálogo**: el `id` (UUID) de
`scraper.productos_crudos`, el mismo que hoy devuelve `GET /api/v1/productos`.

- Cada `producto_id` es **un producto en una sucursal concreta** (la tabla es
  única por `sucursal_id` + `sku`). La ingesta conserva ese id entre una
  extracción y otra.
- Listas **no** sabe nada del EAN ni de equivalencias. Cuando Rutas necesita
  comparar el mismo producto en otras tiendas, le pide a Catálogo que resuelva
  los equivalentes a partir del `producto_id` (SUP-244 y SUP-317).
- **Sin clave foránea entre esquemas.** El `producto_id` se guarda como valor y
  se valida contra Catálogo por HTTP (SUP-312). Motor nunca lee tablas de
  `scraper`.

**Por qué no el EAN directo:** los productos sin EAN no se podrían agregar a una
lista, y Listas quedaría acoplado a un dato interno de Catálogo.

**Cambio respecto de hoy:** el campo actual `producto_normalizado_id` apunta a
`api.productos_normalizados`, una tabla que nada llena. Se reemplaza por
`producto_id`. El frontend todavía no consume artículos (SUP-252 pendiente), así
que el cambio de nombre no rompe ninguna pantalla. La migración de datos está en
la sección 6.

## 2. Qué es la cantidad

- **Número entero de envases**, mayor o igual a 1. "2" significa dos paquetes,
  no dos kilos.
- **Nunca se mezcla con cantidades de receta** (gramos, tazas). Convertir
  "500 g de harina" en "1 paquete de 1 kg" es trabajo de la IA (SUP-325) y el
  usuario lo confirma en pantalla (SUP-329) antes de guardar.
- El máximo por artículo y el máximo de artículos por lista son **configurables**
  (variables de entorno de Motor), no valores fijos en el código.

## 3. Endpoints

Todos van bajo la lista del usuario y usan la sesión de la cookie `jwt`, igual
que el CRUD actual. Una lista de otro usuario responde **404**, igual que una
inexistente, para no revelar qué listas existen.

| Método | Ruta | Qué hace |
| --- | --- | --- |
| `POST` | `/api/v1/listas/{lista_id}/articulos` | Agrega un producto. Si ya está en la lista, **suma** la cantidad |
| `PUT` | `/api/v1/listas/{lista_id}/articulos/{articulo_id}` | Cambia la cantidad y/o marca como comprado |
| `DELETE` | `/api/v1/listas/{lista_id}/articulos/{articulo_id}` | Quita el artículo |
| `POST` | `/api/v1/listas/{lista_id}/articulos/lote` | Agrega varios de una vez, todo o nada (SUP-318) |
| `GET` | `/api/v1/listas/{lista_id}` | Detalle de la lista con sus artículos (ya existe; cambia la forma del artículo) |

### 3.1 Agregar un producto

```http
POST /api/v1/listas/3f6c.../articulos
Content-Type: application/json

{ "producto_id": "9b2e4c1a-7d3f-4e8a-b1c2-5f6a7b8c9d0e", "cantidad": 2 }
```

- **201 Created** si el producto no estaba en la lista.
- **200 OK** si ya estaba: se suma la cantidad y se devuelve el artículo
  actualizado. Sumar es lo que espera el usuario cuando agrega dos veces lo
  mismo.

```json
{
  "id": 41,
  "producto_id": "9b2e4c1a-7d3f-4e8a-b1c2-5f6a7b8c9d0e",
  "cantidad": 2,
  "esta_comprado": false
}
```

Antes de guardar, Motor consulta a Catálogo (SUP-312) que el producto exista y
tenga stock. Si Catálogo no responde, **no se guarda nada** (ver errores).

### 3.2 Cambiar cantidad o marcar comprado

```http
PUT /api/v1/listas/3f6c.../articulos/41
Content-Type: application/json

{ "cantidad": 3 }
```

Acepta `cantidad`, `esta_comprado` o ambos; al menos uno es obligatorio, igual
que `PUT` de la lista. Responde **200** con el artículo actualizado. No consulta
a Catálogo: cambiar la cantidad no cambia el producto.

### 3.3 Quitar

```http
DELETE /api/v1/listas/3f6c.../articulos/41
```

Responde **204** sin cuerpo.

### 3.4 Agregar en lote

Lo usa la pantalla de confirmación de receta (SUP-329) y el carrito.

```http
POST /api/v1/listas/3f6c.../articulos/lote
Content-Type: application/json
Idempotency-Key: 6a1f0d2e-3b4c-4d5e-8f90-a1b2c3d4e5f6

{
  "articulos": [
    { "producto_id": "9b2e4c1a-7d3f-4e8a-b1c2-5f6a7b8c9d0e", "cantidad": 1 },
    { "producto_id": "1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f", "cantidad": 2 }
  ]
}
```

- **Todo o nada.** Si un producto no existe o no tiene stock, no se guarda
  ninguno y la respuesta indica cuáles fallaron.
- **`Idempotency-Key` obligatorio.** Si el frontend reintenta el mismo envío (por
  ejemplo, porque se cortó la conexión), Motor reconoce la clave y devuelve el
  mismo resultado sin sumar las cantidades dos veces. El Gateway ya respeta esta
  cabecera en las escrituras.
- Un mismo `producto_id` repetido dentro del lote se suma antes de guardar.

**200 OK**: devuelve la lista completa con su detalle actualizado (sección 3.5).

**409 Conflict**, si algún producto no se puede agregar:

```json
{
  "estado": 409,
  "mensaje": "No se pudo agregar el lote.",
  "detalle": "Productos no disponibles: 1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f (sin_stock)."
}
```

### 3.5 Detalle de la lista

`GET /api/v1/listas/{lista_id}` mantiene su forma actual, con un cambio en cada
artículo: además de lo que guarda Listas, incluye los datos del producto que
devuelve Catálogo, **solo para mostrarlos**.

```json
{
  "id": "3f6c...",
  "nombre": "Compras de la semana",
  "estado": "activa",
  "total_articulos": 1,
  "creada_el": "2026-11-07T14:02:11Z",
  "actualizada_el": "2026-11-07T14:05:40Z",
  "articulos": [
    {
      "id": 41,
      "producto_id": "9b2e4c1a-7d3f-4e8a-b1c2-5f6a7b8c9d0e",
      "cantidad": 2,
      "esta_comprado": false,
      "estado_producto": "disponible",
      "producto": {
        "nombre": "Leche entera 1 L",
        "marca": "Colún",
        "supermercado": "Lider",
        "unidad": "1 L",
        "precio": 1190,
        "url_imagen": "https://..."
      }
    }
  ]
}
```

`estado_producto` puede ser:

| Valor | Significado | `producto` |
| --- | --- | --- |
| `disponible` | Existe y tiene stock | Con datos |
| `sin_stock` | Existe, pero hoy no tiene stock | Con datos |
| `no_encontrado` | Catálogo ya no lo tiene (dejó de extraerse) | `null` |
| `desconocido` | Catálogo no respondió a tiempo | `null` |

Si Catálogo está caído, **la lista igual se muestra** con
`estado_producto: "desconocido"`: el usuario no pierde su lista por una falla de
otro servicio. Un artículo `no_encontrado` no se borra solo; el usuario decide.

## 4. Errores

Mismo envoltorio que hoy usan todos los servicios: `estado`, `mensaje` y
`detalle` opcional.

| Situación | Código | `mensaje` |
| --- | --- | --- |
| Sin sesión o sesión inválida | 401 | (el de la validación de sesión actual) |
| Lista o artículo inexistente, o de otro usuario | 404 | `La lista no existe.` / `El artículo no existe.` |
| Cuerpo inválido: cantidad < 1, campos extra, `producto_id` mal formado | 400 | `Solicitud inválida.` |
| Lote sin `Idempotency-Key` | 400 | `Solicitud inválida.` |
| Producto inexistente en Catálogo | 422 | `El producto no existe en el catálogo.` |
| Producto sin stock | 409 | `El producto no tiene stock.` |
| Se supera el máximo de artículos o de cantidad | 409 | `Alcanzaste el máximo de artículos en la lista.` |
| Catálogo no responde al agregar | 503 | `No se pudo verificar el producto. Intenta de nuevo.` |

**Regla:** si Motor no pudo verificar un producto, no lo guarda. Un error 503
nunca deja datos a medias.

## 5. Qué usa cada servicio

| Servicio | Tarea | Qué necesita de este contrato |
| --- | --- | --- |
| **Catálogo** (Marcelo) | SUP-312 | Una consulta interna en lote por `producto_id` que devuelva, por cada id: si existe, `en_stock`, `nombre`, `marca`, `supermercado`, `unidad`, `precio`, `url_imagen` y fecha de captura. Con token de servicio, no expuesta por el Gateway. **La forma exacta la define Marcelo en SUP-312.** |
| **Catálogo** (Marcelo) | SUP-244 / SUP-317 | Recibir `producto_id` y resolver sus equivalentes por EAN en otras sucursales |
| **IA** (Renato) | SUP-325 | Devolver candidatos con `producto_id` de Catálogo y una cantidad en envases, listos para el lote |
| **Rutas** (Vicho) | SUP-316 | Leer de la lista los pares `producto_id` + `cantidad` |
| **Frontend** (Daniela) | SUP-252 / SUP-329 | Los endpoints de la sección 3, `estado_producto` para mostrar alertas y `Idempotency-Key` en el lote |

## 6. Migración de los datos actuales (SUP-327)

- Agregar la columna `producto_id UUID` a `rutas.articulos_lista_compras`, sin
  clave foránea.
- `producto_normalizado_id` deja de usarse y se elimina con su FK hacia
  `api.productos_normalizados`, porque esa tabla nunca se llenó.
- Antes de aplicar, **contar en una base de prueba** cuántos artículos existen.
  Lo esperado es cero o casi cero; si hay alguno, se conserva marcado como no
  resoluble en vez de inventarle un `producto_id`.
- Ampliar los permisos de `app_rutas` sobre la tabla (hoy solo tiene `SELECT`) y
  actualizar la matriz de `06_propiedad_permisos.sql` y
  `smoke_data_ownership.py`.
- Un índice único por (`lista_id`, `producto_id`) garantiza la regla de "sumar si
  ya existe".

## 7. Fuera de alcance

- Equivalencias por nombre o por IA: solo EAN exacto (SUP-244).
- Unidades de receta (gramos, porciones) dentro de la lista.
- Historial de precios en la lista: el precio mostrado es el actual de Catálogo.
- Escrituras simultáneas desde dos pestañas (SUP-343, Could).

## Preguntas abiertas

- [ ] **Marcelo:** ¿la consulta en lote de SUP-312 puede devolver todos los
  campos de la sección 5 en una sola llamada?
- [ ] **Marcelo:** ¿el `id` de `productos_crudos` se mantiene si el scraper deja
  de ver un producto y después vuelve a encontrarlo?
- [ ] **Renato:** ¿la IA puede proponer la cantidad en envases a partir de la
  presentación que devuelve Catálogo, o solo el producto?
- [ ] **Equipo:** ¿agregar un producto sin stock debe rechazarse (propuesta
  actual, 409) o permitirse con una advertencia, para planificar la compra?
- [ ] **Equipo:** valores iniciales para el máximo de cantidad por artículo y de
  artículos por lista.
