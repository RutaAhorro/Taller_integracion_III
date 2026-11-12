# Contrato del motor de rutas

> **Estado:** propuesta en revisión · SUP-323 · Responsable: Vicho
> **Revisan:** Marcelo (Catálogo, SUP-317), Daniela (frontend, SUP-330 y SUP-261) y Esban (Listas y migración a Go, SUP-241 y SUP-348)

Esta es una **propuesta inicial** para ratificar en la revisión del PR. No es
una decisión ya tomada en una reunión. Una vez fusionada en `develop`, cambiarla
sigue la misma regla que el [contrato de artículos](CONTRATO_ARTICULOS_LISTA.md):
se avisa antes en el canal del equipo y el PR lo aprueba quien consume la parte
afectada.

> **Distancias y costos estimados.** El motor no calcula rutas por calles reales.
> La distancia es una línea recta (Haversine) multiplicada por un factor de
> corrección declarado, y el costo de traslado se estima con un precio de
> combustible regional. Toda respuesta del motor lo indica y el frontend debe
> mostrarlo.

## Para qué existe

El motor responde una pregunta: **dada una lista guardada y un punto de
partida, ¿en qué tiendas conviene comprar, en qué orden visitarlas y cuánto
cuesta en total?** Este contrato fija los límites del cálculo, las reglas que
usa, qué le pide a Catálogo y qué le responde al frontend. El algoritmo se
implementa en SUP-313 (tiendas y asignación) y SUP-328 (orden y traslado). El
endpoint se expone en SUP-316.

## 1. Alcance del cálculo (MVP del Sprint 3)

| Regla | Valor |
| --- | --- |
| Origen | Coordenadas (`lat`, `lon`) que envía el frontend en cada solicitud. No se guarda. |
| Retorno | **El recorrido vuelve al origen**: origen → tiendas → origen. |
| Paradas | Entre 1 y `RUTAS_MAX_PARADAS` tiendas (valor inicial: 3). |
| Candidatos | Como máximo `RUTAS_MAX_SUCURSALES` sucursales (valor inicial: 10), las más cercanas al origen dentro del radio. |
| Artículos | Como máximo `RUTAS_MAX_ARTICULOS` artículos por cálculo (valor inicial: 30). |
| Distancia | Haversine × `RUTAS_FACTOR_DISTANCIA` (valor inicial: 1,3). |
| Umbral de ahorro | Una ruta con varias tiendas solo se recomienda si ahorra más de `RUTAS_UMBRAL_AHORRO` pesos frente a comprar todo en una tienda (valor inicial: $500). Regla RN-08. |
| Transporte | Solo auto particular. Transporte público (OTP) queda fuera del Sprint 3. |
| Perfil | No se usa el perfil avanzado ni el plan premium. |

Los límites son **variables de entorno de Motor**, no valores fijos en el
código. Se registran en el README del servicio al implementarlos (SUP-316).
Los "valores iniciales" son la propuesta para el entorno de desarrollo.

Con estos límites, el cálculo prueba todas las combinaciones: hay como máximo
175 conjuntos de tiendas (10 de una, 45 de dos y 120 de tres) y 6 órdenes de
visita por conjunto. No hace falta OR-Tools ni una heurística (ver
[D11](../Decisiones/11_Motor_de_rutas_en_Go.md)). Si en el futuro se exigen
calles reales, transporte público o más paradas, es un cambio de alcance: se
registra y se reestima antes de programarlo.

## 2. Cómo se calcula

### 2.1 Qué se compra

- Se leen los artículos de la lista con `esta_comprado = false`, como pares
  `producto_id` + `cantidad` en envases ([contrato de artículos](CONTRATO_ARTICULOS_LISTA.md), sección 2).
- Catálogo resuelve, para cada `producto_id`, **las ofertas equivalentes por EAN
  exacto** en las sucursales candidatas (SUP-244, SUP-317). Un producto sin EAN
  solo se compara consigo mismo, en su propia sucursal. No hay equivalencia por
  nombre.
- La cantidad no se convierte: el mismo EAN implica el mismo envase.

### 2.2 Qué tiendas y qué se compra en cada una (SUP-313)

1. **Productos no cubiertos.** Un artículo sin ninguna oferta con stock en las
   candidatas se marca como `no_cubierto`, sale del cálculo y se informa en la
   respuesta. El resto se sigue calculando.
2. **Conjuntos.** Se prueban todos los conjuntos de 1 a `RUTAS_MAX_PARADAS`
   tiendas.
3. **Asignación.** En cada conjunto, cada artículo se compra en la oferta con
   stock más barata del conjunto. Costo del artículo = `precio × cantidad`.
4. **Alternativas incompletas.** Si un conjunto no cubre todos los artículos
   cubribles, se descarta. Un conjunto con una tienda donde no se compra nada
   también se descarta, porque ya se evaluó el conjunto sin esa tienda.

### 2.3 Orden de visita y traslado (SUP-328)

- Para cada conjunto se prueban todos los órdenes de visita y se elige el de
  menor distancia total: origen → tiendas → origen.
- **Costo de traslado** (regla de negocio, [Reglas de Negocio](../Requisitos/Reglas_de_Negocio.md)):

  ```text
  distancia_km   = Σ Haversine(tramo) × RUTAS_FACTOR_DISTANCIA
  costo_traslado = (distancia_km ÷ rendimiento_km_l) × precio_combustible_l
  total          = costo_productos + costo_traslado
  ```

- `rendimiento_km_l`: lo envía el frontend en la solicitud. Si no lo envía, se
  usa `RUTAS_RENDIMIENTO_KM_L_DEFECTO` (valor inicial: 12 km/L).
- `precio_combustible_l`: precio regional semanal de la gasolina de 93 octanos
  según Energía Abierta (CNE), guardado en caché. Si la fuente no responde, se
  usa `RUTAS_PRECIO_COMBUSTIBLE_RESPALDO` y la respuesta lo indica con
  `fuente: "respaldo"`. Antes de implementarlo hay que verificar que la fuente
  responde fuera del navegador y revisar sus condiciones de uso.
- Los montos se redondean al peso entero solo al final. Las distancias se
  informan con dos decimales.

### 2.4 Cuál alternativa gana

Gana la de **menor `total`**. Si hay empate, el desempate es determinista y
sigue este orden:

1. Menos paradas.
2. Menor distancia.
3. Menor lista de `sucursal_id`, ordenada de forma ascendente.

**Umbral de ahorro (RN-08).** Si la ganadora tiene varias tiendas y existe una
base de una tienda (sección 2.5), se compara su ahorro con el umbral:

```text
si base.total − ganadora.total <= RUTAS_UMBRAL_AHORRO  →  se recomienda la base
```

Visitar más tiendas cansa y el traslado es solo una estimación, así que un
ahorro pequeño no justifica la ruta. Si no hay base, porque ninguna tienda sola
cubre la compra, el umbral no aplica y se mantiene la ganadora. Con
`RUTAS_UMBRAL_AHORRO=0` se recomienda cualquier ahorro positivo. Ver el
[caso C](#caso-c-el-ahorro-no-supera-el-umbral).

En el orden de visita, un recorrido y su inverso miden lo mismo. En ese caso se
elige el que **visita primero la tienda más cercana al origen**.

### 2.5 Base de comparación y ahorro

- **Base:** la mejor alternativa de **una sola tienda** que cubra lo mismo que
  la alternativa elegida, incluido su traslado.
- `ahorro = base.total − elegida.total`. Si la elegida es de una tienda, el
  ahorro es 0. Esto incluye el caso en que el umbral (sección 2.4) descartó una
  combinación: la respuesta recomienda la base y no informa el ahorro descartado.
- Si ninguna tienda sola cubre la compra, `base` es `null` y no se informa
  ahorro. **No se inventa un precio de referencia.**
- La alternativa es la mejor **entre los candidatos y reglas evaluados**, no
  entre todas las tiendas que existen. La respuesta no debe prometer otra cosa.

## 3. Endpoint público (SUP-316)

```http
POST /api/v1/rutas/optimizar
Content-Type: application/json
Cookie: jwt=...
```

Lo expone Motor y lo enruta el Gateway con el prefijo `/api/v1/rutas*`, sin
lógica de negocio. La sesión es la misma cookie `jwt` del CRUD de listas.

### 3.1 Solicitud

```json
{
  "lista_id": "3f6c2a10-8b1e-4d2f-9a3c-7e5d4c3b2a10",
  "origen": { "lat": -38.739, "lon": -72.598 },
  "rendimiento_km_l": 12
}
```

| Campo | Obligatorio | Regla |
| --- | --- | --- |
| `lista_id` | Sí | UUID de una lista del usuario de la sesión |
| `origen.lat` | Sí | Número entre −90 y 90 |
| `origen.lon` | Sí | Número entre −180 y 180 |
| `rendimiento_km_l` | No | Número mayor que 0. Si falta, se usa el valor por defecto |

Los campos extra se rechazan con 400, igual que en el CRUD de listas. El radio
de búsqueda no lo elige el usuario: es `RUTAS_RADIO_KM` (valor inicial: 10 km).

### 3.2 Respuesta 200

Ver el [caso B](#caso-b-conviene-combinar-dos-tiendas-y-un-producto-no-se-encuentra)
para un ejemplo completo. La forma general es:

| Campo | Qué es |
| --- | --- |
| `estimado` | Siempre `true`. Recuerda que distancias y costos son aproximados |
| `parametros` | Valores usados: factor de distancia, rendimiento, umbral de ahorro y combustible (`precio_l`, `fuente`, `fecha`) |
| `ruta` | La alternativa elegida: `paradas` en orden, `tramos`, `distancia_km`, `costo_productos`, `costo_traslado` y `total` |
| `ruta.paradas[]` | `orden`, `sucursal_id`, `nombre`, `cadena`, `lat`, `lon`, `subtotal` y `articulos` (con `producto_id` de la lista, `producto_id_oferta`, `cantidad`, `precio_unitario` y `subtotal`) |
| `base_una_tienda` | `sucursal_id`, `nombre`, `total`; o `null` si ninguna tienda sola alcanza |
| `ahorro` | Número; o `null` cuando `base_una_tienda` es `null` |
| `no_cubiertos[]` | `producto_id` y `motivo` (`sin_oferta_en_radio`) de los artículos que salieron del cálculo |
| `candidatas_evaluadas` | Cantidad de sucursales consideradas |

`producto_id_oferta` es el `producto_id` del equivalente que se compra en esa
sucursal. Puede ser distinto del guardado en la lista cuando Catálogo resolvió
un equivalente por EAN en otra tienda.

### 3.3 Errores

Mismo envoltorio que el resto de los servicios: `estado`, `mensaje` y `detalle`
opcional.

| Situación | Código | `mensaje` |
| --- | --- | --- |
| Sin sesión o sesión inválida | 401 | (el de la validación de sesión actual) |
| Lista inexistente o de otro usuario | 404 | `La lista no existe.` |
| Cuerpo inválido, coordenadas fuera de rango, campos extra | 400 | `Solicitud inválida.` |
| Lista sin artículos pendientes | 422 | `La lista no tiene artículos por comprar.` |
| Más artículos que `RUTAS_MAX_ARTICULOS` | 422 | `La lista supera el máximo de artículos para calcular una ruta.` |
| No hay sucursales con coordenadas dentro del radio | 422 | `No hay supermercados cercanos para calcular una ruta.` |
| Ningún artículo tiene oferta en el radio, o ningún conjunto de hasta `RUTAS_MAX_PARADAS` tiendas cubre la compra | 422 | `No se encontró una forma de comprar la lista en las tiendas cercanas.` (el `detalle` indica los productos sin cobertura) |
| Catálogo no responde o responde con error | 503 | `No se pudo consultar precios. Intenta de nuevo.` |

**Regla:** si falta un dato, el motor no inventa una ruta. Un 422 o un 503 no
devuelve una ruta parcial.

## 4. Qué le pide Rutas a Catálogo (SUP-317)

Consulta interna con token de servicio, no expuesta por el Gateway. **La forma
exacta la define Marcelo en SUP-317.** Esta sección fija lo que Rutas necesita
para calcular.

**Rutas envía:**

- Los `producto_id` de la lista, en lote.
- El origen (`lat`, `lon`), el radio en km y el máximo de sucursales.

**Rutas necesita recibir:**

- Por cada **sucursal candidata**: `sucursal_id`, `nombre`, `cadena`, `lat` y
  `lon`. Solo sucursales físicas y activas, con coordenadas reales.
- Por cada **oferta**: `producto_id` solicitado, `producto_id_oferta`,
  `sucursal_id`, `precio` (CLP por envase), `en_stock` y fecha de captura.
- Los `producto_id` **sin ninguna oferta** en el radio, en una lista aparte.
  Una ausencia debe ser explícita, nunca un silencio.

**Lo que Catálogo no debe hacer:**

- Inventar coordenadas. Una sucursal sin coordenadas reales se excluye y se
  informa.
- Incluir tiendas solo en línea. En el `init.sql` actual, A Cuenta y Cugat
  tienen la dirección "Casa Matriz / Online" con las coordenadas del centro de
  Temuco. Para el motor no son paradas visitables (ver preguntas abiertas).
- Equiparar productos por nombre.

Ejemplo **ilustrativo** de la respuesta, para que Marcelo lo ajuste:

```json
{
  "sucursales": [
    { "sucursal_id": 1, "nombre": "Jumbo Los Pablos", "cadena": "Jumbo", "lat": -38.741, "lon": -72.6 },
    { "sucursal_id": 2, "nombre": "Lider Prieto Norte", "cadena": "Lider", "lat": -38.735, "lon": -72.59 }
  ],
  "ofertas": [
    {
      "producto_id": "9b2e4c1a-7d3f-4e8a-b1c2-5f6a7b8c9d0e",
      "producto_id_oferta": "9b2e4c1a-7d3f-4e8a-b1c2-5f6a7b8c9d0e",
      "sucursal_id": 1,
      "precio": 990,
      "en_stock": true,
      "capturado_el": "2026-11-10T06:12:00Z"
    }
  ],
  "sin_ofertas": ["5e6f7a8b-9c0d-4e1f-a2b3-c4d5e6f7a8b9"]
}
```

## 5. Casos de resultado esperado

Los dos casos usan las coordenadas de sucursales de `infrastructure/db/init.sql`
y precios **inventados para el ejemplo**. Sirven como pruebas del algoritmo
(SUP-313 y SUP-328) con datos que crea el propio test, no como datos de la
aplicación.

Parámetros comunes:

- Origen: `-38.739, -72.598`.
- Factor de distancia: 1,3.
- Rendimiento: 12 km/L.
- Combustible: $1.300/L (`fuente: "respaldo"`).

| Sucursal | `sucursal_id` | Distancia en línea recta desde el origen |
| --- | --- | --- |
| Jumbo Los Pablos | 1 | 0,28 km |
| Lider Prieto Norte | 2 | 0,82 km |
| Unimarc Av. Alemania | 3 | 1,24 km |

### Caso A: una tienda alcanza

Lista: leche ×2, arroz ×1, aceite ×1.

| Producto | Jumbo (1) | Lider (2) | Unimarc (3) |
| --- | --- | --- | --- |
| Leche | 1.150 | 1.190 | 1.250 |
| Arroz | 1.690 | 1.590 | 1.650 |
| Aceite | 2.890 | 2.990 | 3.190 |

| Alternativa | Productos | Distancia | Traslado | Total |
| --- | --- | --- | --- | --- |
| **Jumbo** | 6.880 | 0,73 km | 79 | **6.959** |
| Jumbo → Lider | 6.780 | 2,86 km | 310 | 7.090 |
| Jumbo → Unimarc | 6.840 | 3,24 km | 351 | 7.191 |
| Lider | 6.960 | 2,14 km | 232 | 7.192 |
| Unimarc | 7.340 | 3,21 km | 348 | 7.688 |

**Resultado esperado:** comprar todo en Jumbo, con un total de $6.959. Comprar
el arroz en Lider ahorra $100 en productos, pero el traslado extra cuesta $231.
La base de una tienda es la misma alternativa, así que el `ahorro` es 0 y
`no_cubiertos` queda vacío.

### Caso B: conviene combinar dos tiendas y un producto no se encuentra

Lista: leche ×2, arroz ×1, detergente ×1, quinoa ×1.

| Producto | Jumbo (1) | Lider (2) | Unimarc (3) |
| --- | --- | --- | --- |
| Leche | 690 | 1.190 | — |
| Arroz | 1.690 | 1.590 | 1.490 |
| Detergente | 9.990 | 6.990 | 7.490 |
| Quinoa | — | — | — |

| Alternativa | Productos | Distancia | Traslado | Total |
| --- | --- | --- | --- | --- |
| **Jumbo → Lider** | 9.960 | 2,86 km | 310 | **10.270** |
| Lider → Jumbo → Unimarc | 9.860 | 5,37 km | 582 | 10.442 |
| Jumbo → Unimarc | 10.360 | 3,24 km | 351 | 10.711 |
| Lider (base de una tienda) | 10.960 | 2,14 km | 232 | 11.192 |
| Lider → Unimarc | 10.860 | 5,36 km | 580 | 11.440 |
| Jumbo | 13.060 | 0,73 km | 79 | 13.139 |

Cada orden de visita ya aplica el desempate de la sección 2.4. En la ruta
elegida, Jumbo va primero porque es la tienda más cercana al origen.

**Resultado esperado:**

- Quinoa sale como `no_cubierto` y no se calcula.
- Se compran la leche en Jumbo y el arroz y el detergente en Lider, con un total
  de $10.270.
- La tercera parada en Unimarc ahorra $100 en arroz, pero cuesta $272 más de
  traslado, así que no conviene.
- La base es Lider ($11.192), por lo que el ahorro es de $922. Supera el umbral
  de $500, así que se recomienda la combinación.

```json
{
  "estimado": true,
  "parametros": {
    "factor_distancia": 1.3,
    "rendimiento_km_l": 12,
    "umbral_ahorro": 500,
    "combustible": { "precio_l": 1300, "fuente": "respaldo", "fecha": null }
  },
  "ruta": {
    "paradas": [
      {
        "orden": 1, "sucursal_id": 1, "nombre": "Jumbo Los Pablos", "cadena": "Jumbo",
        "lat": -38.741, "lon": -72.6, "subtotal": 1380,
        "articulos": [
          { "producto_id": "…leche", "producto_id_oferta": "…leche-jumbo", "cantidad": 2, "precio_unitario": 690, "subtotal": 1380 }
        ]
      },
      {
        "orden": 2, "sucursal_id": 2, "nombre": "Lider Prieto Norte", "cadena": "Lider",
        "lat": -38.735, "lon": -72.59, "subtotal": 8580,
        "articulos": [
          { "producto_id": "…arroz", "producto_id_oferta": "…arroz-lider", "cantidad": 1, "precio_unitario": 1590, "subtotal": 1590 },
          { "producto_id": "…detergente", "producto_id_oferta": "…detergente-lider", "cantidad": 1, "precio_unitario": 6990, "subtotal": 6990 }
        ]
      }
    ],
    "tramos": [
      { "desde": "origen", "hasta": 1, "distancia_km": 0.37 },
      { "desde": 1, "hasta": 2, "distancia_km": 1.42 },
      { "desde": 2, "hasta": "origen", "distancia_km": 1.07 }
    ],
    "distancia_km": 2.86,
    "costo_productos": 9960,
    "costo_traslado": 310,
    "total": 10270
  },
  "base_una_tienda": { "sucursal_id": 2, "nombre": "Lider Prieto Norte", "total": 11192 },
  "ahorro": 922,
  "no_cubiertos": [ { "producto_id": "…quinoa", "motivo": "sin_oferta_en_radio" } ],
  "candidatas_evaluadas": 3
}
```

Los `producto_id` del ejemplo se abrevian; en la respuesta real son UUID.

### Caso C: el ahorro no supera el umbral

Misma lista y precios del caso B, salvo la leche en Jumbo, que vuelve a costar
$990.

| Alternativa | Productos | Distancia | Traslado | Total |
| --- | --- | --- | --- | --- |
| Jumbo → Lider | 10.560 | 2,86 km | 310 | 10.870 |
| Lider → Jumbo → Unimarc | 10.460 | 5,37 km | 582 | 11.042 |
| **Lider (base de una tienda)** | 10.960 | 2,14 km | 232 | **11.192** |
| Jumbo → Unimarc | 10.960 | 3,24 km | 351 | 11.311 |
| Lider → Unimarc | 10.860 | 5,36 km | 580 | 11.440 |
| Jumbo | 13.660 | 0,73 km | 79 | 13.739 |

**Resultado esperado:**

- Por total gana Jumbo → Lider ($10.870), pero solo ahorra $322 frente a Lider.
- $322 no supera el umbral de $500, así que se recomienda comprar todo en Lider,
  con un total de $11.192.
- `ruta` tiene una sola parada, `base_una_tienda` es Lider, `ahorro` es 0 y
  quinoa sigue como `no_cubierto`.

## 6. Qué usa cada servicio

| Servicio | Tarea | Qué necesita de este contrato |
| --- | --- | --- |
| **Rutas** (Vicho) | SUP-313, SUP-328 | Reglas de las secciones 1 y 2 y los casos de la sección 5 como pruebas. El umbral de ahorro (RN-08, sección 2.4) se implementa en SUP-328 |
| **Rutas** (Vicho) | SUP-316 | Endpoint de la sección 3 y consulta a Catálogo de la sección 4 |
| **Catálogo** (Marcelo) | SUP-317 | Sección 4: qué enviar, qué no inventar y qué informar como faltante |
| **Listas** (Esban) | SUP-241, SUP-348 | Rutas lee `producto_id` + `cantidad` de los artículos sin comprar; la migración a Go conserva este endpoint |
| **Frontend** (Daniela) | SUP-330, SUP-261 | Solicitud y respuesta de la sección 3, leyenda de valores estimados y `no_cubiertos` para avisar al usuario |
| **Gateway** | SUP-316 | Enrutar `/api/v1/rutas*` hacia `RUTAS_URL`, sin lógica de negocio |

## 7. Fuera de alcance

- Calles reales (OSRM), tráfico, transporte público (OTP) y tiempos de viaje.
- Guardar el historial de cálculos. Las tablas `rutas.ejecuciones_optimizacion`
  y `rutas.paradas_optimizacion` existen, pero esta versión no las escribe.
- Perfil de transporte del usuario (`api.perfiles_transporte_usuario` es de
  Identidad), premium y límites por plan.
- Horarios de apertura de las sucursales.
- Comprar parte de un artículo en una tienda y el resto en otra.
- **Monto mínimo de compra (RN-20).** Se difiere: no se rechazan listas por
  subtotal bajo. Con el umbral de ahorro, una lista pequeña ya termina en una
  sola tienda, y rechazarla obligaría a un error nuevo y a que el frontend
  sugiera agregar productos. Se puede retomar en un sprint posterior.

## Preguntas abiertas

- [ ] **Equipo:** ¿el recorrido vuelve al origen (propuesta actual) o termina en
  la última tienda?
- [ ] **Equipo:** ¿se calcula la ruta sin los productos no cubiertos (propuesta
  actual) o se rechaza toda la lista con 422?
- [ ] **Equipo:** valores iniciales de radio (10 km), factor (1,3), rendimiento
  (12 km/L), umbral de ahorro ($500) y combustible de respaldo.
- [ ] **Marcelo:** ¿Catálogo puede distinguir sucursales físicas de tiendas solo
  en línea (A Cuenta y Cugat)? Si no, ¿se excluyen por cadena mientras tanto?
- [ ] **Marcelo:** las coordenadas de sucursales del `init.sql` parecen
  aproximadas. ¿Hay coordenadas reales para la demo (SUP-326)?
- [ ] **Marcelo y Renato:** ¿Motor usa `CATALOGO_INTERNAL_TOKEN`? Ese token hoy
  autoriza la ingesta, así que reutilizarlo le daría a Motor permiso de
  escritura. Conviene un token de lectura propio, con nombre acordado según la
  [guía de configuración](../Guias/GUIA_CONFIGURACION.md).
