# Contrato de equivalencia exacta por EAN

> **Estado:** vigente al fusionarse en `develop` · SUP-244 · Responsable: Marcelo
> **Consumen:** Rutas (SUP-316 y SUP-317) y, de forma indirecta, Listas e IA
> **Aprueban cambios:** Vicho (Rutas) y Marcelo (Catálogo)

Este contrato fija cómo Catálogo decide que dos productos de sucursales
distintas son el mismo artículo. Es la base de SUP-317: primero se resuelve la
equivalencia y después se agregan coordenadas, radio y cálculo de ruta. La forma
exacta del endpoint de SUP-317 se define en ese ticket; aquí sólo se garantiza
la identidad del producto y su oferta actual.

## Para qué existe

Un producto de Lider y uno de Jumbo pueden ser el mismo artículo con distinto
`producto_id`, nombre y sucursal. Sin una clave común, el motor de rutas no puede
comparar precios. La clave es el **EAN exacto**: no se usan nombres, IA,
homologación manual ni coincidencia difusa.

Reglas que este contrato no rompe:

- No crea tablas, columnas ni variables de entorno.
- No usa la tabla legada `api.equivalencias_productos`.
- No inventa un EAN ni corrige un dígito verificador.
- No devuelve coordenadas: eso pertenece a SUP-317.

## 1. Endpoint

```http
POST /internal/v1/productos/equivalencias
Authorization: Bearer <CATALOGO_CONSULTA_TOKEN>
Content-Type: application/json
```

Ruta interna de sólo lectura. No se publica en el Gateway ni en un Ingress
público. Comparte con la consulta de disponibilidad el token
`CATALOGO_CONSULTA_TOKEN`, el límite de lote `CATALOGO_MAX_PRODUCTOS_CONSULTA`
(1000 por defecto) y el cuerpo JSON estricto: se rechazan campos desconocidos,
contenido extra después del JSON y cuerpos mayores a 8 MiB.

## 2. Solicitud

```json
{
  "producto_ids": [
    "9b2e4c1a-7d3f-4e8a-b1c2-5f6a7b8c9d0e",
    "1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f"
  ]
}
```

| Campo | Obligatorio | Regla |
| --- | --- | --- |
| `producto_ids` | Sí | Entre 1 y `CATALOGO_MAX_PRODUCTOS_CONSULTA` UUID válidos |

- Deduplica los UUID y conserva el orden de la primera aparición.
- Un UUID inválido rechaza el lote completo con 400.
- Devuelve un elemento por producto solicitado, incluso si no existe en la base.

## 3. Normalización del EAN

1. Se conservan sólo los dígitos del valor almacenado en `ean_gtin`; los
   separadores se descartan. El código se compara como texto para no perder
   ceros iniciales.
2. Sólo son válidos los largos GTIN de 8, 12, 13 o 14 dígitos con dígito
   verificador módulo 10 correcto.
3. El código válido se canoniza a 14 dígitos rellenando con ceros a la
   izquierda. Así un UPC-A de 12 dígitos (`036000291452`) y su EAN-13 con cero
   inicial (`0036000291452`) producen la misma clave `00036000291452`.
4. Un código con largo distinto o dígito verificador incorrecto es inválido y
   nunca se usa para unir productos.

## 4. Estados

| Valor | Significado | Ofertas devueltas |
| --- | --- | --- |
| `comparable` | Tiene EAN válido y el grupo no presenta conflicto | Todas las sucursales del grupo |
| `sin_ean` | No tiene `ean_gtin` o está vacío | Sólo la oferta del propio producto |
| `ean_invalido` | Tiene código, pero el largo o el dígito verificador no son válidos | Sólo la oferta del propio producto |
| `conflicto_presentacion` | El grupo comparte EAN, pero tiene formatos explícitos distintos | Sólo la oferta del propio producto |
| `no_encontrado` | El UUID no existe en `productos_crudos` | `[]` |

**Conflicto de presentación.** Se comparan sólo formatos explícitos
(`formato_crudo`) normalizados a minúsculas y espacios simples. Dos formatos no
vacíos distintos, por ejemplo `1 L` y `1.5 L`, marcan el grupo como conflictivo.
Un formato vacío no conflictúa porque no prueba una presentación distinta.

## 5. Respuesta

```json
{
  "productos": [
    {
      "producto_id": "9b2e4c1a-7d3f-4e8a-b1c2-5f6a7b8c9d0e",
      "estado": "comparable",
      "ean": "00036000291452",
      "presentacion": "1 L",
      "ofertas": [
        {
          "producto_id_oferta": "1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f",
          "sucursal_id": 1,
          "cadena": "Jumbo",
          "sucursal": "Jumbo Centro",
          "nombre": "Leche entera 1 L",
          "unidad": "1 L",
          "precio": 1190,
          "precio_normal": 1190,
          "precio_oferta": null,
          "en_stock": true,
          "capturado_el": "2026-11-16T10:00:00Z",
          "estado_precio": "vigente"
        }
      ]
    }
  ],
  "resumen": {
    "solicitados": 1,
    "comparables": 1,
    "sin_ean": 0,
    "ean_invalidos": 0,
    "conflictos_presentacion": 0,
    "no_encontrados": 0
  }
}
```

### 5.1 Producto

| Campo | Tipo | Regla |
| --- | --- | --- |
| `producto_id` | UUID | El ID solicitado, en el orden de la primera aparición |
| `estado` | string | Uno de los valores de la sección 4 |
| `ean` | string o `null` | GTIN canónico de 14 dígitos; `null` si no es válido |
| `presentacion` | string | `formato_crudo` sin espacios externos; puede ser vacío |
| `ofertas` | array | Nunca `null`; vacío sólo en `no_encontrado` |

### 5.2 Oferta

| Campo | Tipo | Regla |
| --- | --- | --- |
| `producto_id_oferta` | UUID | ID real de Catálogo que se compra en esa sucursal |
| `sucursal_id` | int | Sucursal de la oferta |
| `cadena` | string | Nombre de la cadena |
| `sucursal` | string | Nombre de la sucursal |
| `nombre` | string | Título crudo del producto |
| `unidad` | string | `formato_crudo`; puede ser vacío |
| `precio` | number o `null` | Precio efectivo vigente, en CLP por envase |
| `precio_normal` | number o `null` | Precio normal de la captura vigente |
| `precio_oferta` | number o `null` | Precio de oferta de la captura vigente |
| `en_stock` | boolean o `null` | `esta_disponible` de la captura vigente |
| `capturado_el` | timestamp o `null` | Fecha de la captura más reciente |
| `estado_precio` | string | `vigente`, `vencido` o `sin_captura` |

`producto_id_oferta` es el mismo campo que espera el motor de rutas en su
respuesta. Puede ser distinto del `producto_id` solicitado cuando el equivalente
se compra en otra sucursal.

**Vigencia.** Se usa la captura más reciente de cada producto. `precio`,
`precio_normal`, `precio_oferta` y `en_stock` sólo se publican si la captura
tiene a lo más `CATALOGO_DISPONIBILIDAD_ANTIGUEDAD_MAXIMA` (48 h por defecto).
Si falta, está vencida o tiene fecha futura, esos campos llegan en `null` y
`estado_precio` es `sin_captura` o `vencido`. `capturado_el` se informa igual
para que el consumidor conozca la fecha del dato.

Las ofertas se ordenan por `sucursal_id` y luego por `producto_id_oferta`.

### 5.3 Resumen

`resumen` cuantifica la cobertura de esa misma consulta: `solicitados`,
`comparables`, `sin_ean`, `ean_invalidos`, `conflictos_presentacion` y
`no_encontrados`. Sirve para informar cuántos productos no se pueden comparar
sin rellenar el caso con datos ficticios. Si la cobertura real de EAN es baja,
el contador lo muestra. Las fechas del ejemplo son ilustrativas: la vigencia se
calcula con la hora de la consulta.

## 6. Errores

Mismo formato que el resto de las rutas internas: `{"error": "<código>"}`, sin
detalle SQL.

| Situación | Código HTTP | `error` |
| --- | --- | --- |
| Sin token o token incorrecto | 401 | `internal_unauthorized` |
| Petición con cabecera `Origin` | 403 | `internal_origin_forbidden` |
| Lote vacío o sobre el máximo | 400 | `invalid_batch` |
| UUID inválido | 400 | `invalid_product_id` |
| JSON inválido, campo extra o contenido extra | 400 | `invalid_payload` |
| Cuerpo mayor a 8 MiB | 413 | `invalid_payload` |
| Fallo interno de Catálogo | 500 | `catalogo_internal_error` |

## 7. Lo que agrega SUP-317

Este contrato termina en la oferta actual por sucursal. SUP-317 añade:

- Coordenadas reales de sucursales físicas y activas.
- Filtro por origen y radio.
- Exclusión explícita de tiendas sólo en línea y de sucursales sin
  coordenadas.
- La lista de `producto_id` sin oferta en el radio.

Rutas no debe inventar coordenadas ni equiparar productos por nombre. Si
Catálogo no puede resolver un producto, debe informarlo con su estado y su
`resumen`, nunca omitirlo en silencio.

## 8. Límites conocidos

- La calidad depende de que el scraper haya extraído el EAN y de que esté
  almacenado sin errores. La consulta informa la cobertura, no la mejora.
- La normalización en SQL (`regexp_replace`) no usa un índice funcional. Añadir
  ese índice sería una migración fuera de SUP-244.
- Un formato vacío no permite detectar un conflicto de presentación; se prefiere
  no bloquear el grupo con un dato que no prueba nada.
- Un mismo EAN puede aparecer en varias sucursales de la misma cadena; cada
  combinación producto-sucursal es una oferta independiente.

## 9. Cambios

Una vez fusionado en `develop`, cambiar este contrato exige avisar antes en el
canal del equipo y abrir un PR que apruebe quien consume la parte afectada.
Rutas y Catálogo son los consumidores directos.
