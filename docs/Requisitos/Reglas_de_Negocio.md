# Reglas de negocio

Políticas y cálculos que el sistema debe respetar. Los identificadores `RN-xx`
son los de la especificación formal del Sprint 0 (v1.3) y los usan los issues
de Linear: **no se renumeran**. Los valores umbral (frescura, máximo de
paradas, ahorro mínimo, monto mínimo) son **configurables**, nunca fijos en el
código.

## Regla central

| ID | Regla | Definición | Estado |
| --- | --- | --- | --- |
| RN-00 | Mejor alternativa | La mejor alternativa de compra minimiza `Costo total = Precio de los productos + Costo de desplazamiento`, con stock confirmado y según el medio de transporte del usuario. El presupuesto y las restricciones dietéticas quedan fuera de alcance | Sprint 3 (SUP-323, SUP-328) |

## Catálogo de reglas

| ID | Regla | Definición | Estado |
| --- | --- | --- | --- |
| RN-01 | Vigencia de precios | Un precio vale durante el período de frescura configurado; después se oculta y no se usa en cálculos | Sprint 3: el umbral se define en SUP-312 |
| RN-02 | Tratamiento del stock | Sin stock, el producto se excluye de la ruta y se muestra una alerta visual; no se elimina en silencio | Sprint 3 (SUP-331, [contrato de artículos](../Contratos/CONTRATO_ARTICULOS_LISTA.md)) |
| RN-06 | Validación de la IA | El backend detecta la falta de stock antes de que la IA redacte, para que no recomiende lo que no existe | Sprint 3 (SUP-325, SUP-315) |
| RN-07 | Límite de locales | La ruta tiene un máximo configurable de tiendas | Sprint 3: máximo 3 (SUP-323) |
| RN-08 | Umbral de rentabilidad | Solo se sugiere una ruta con varias tiendas si el ahorro neto supera el umbral configurado | Sprint 3 (SUP-328) |
| RN-11 | Control de acceso | Invitado (solo lectura), registrado y colaborador. Una acción protegida de un invitado lo lleva a iniciar sesión | Implementado (roles `registrado`, `colaborador`, `admin`). La cuota de IA por rol queda fuera de alcance |
| RN-14 | Capacidad de proceso | El motor rechaza listas que superen la capacidad máxima configurada | Sprint 3 (SUP-316) |
| RN-20 | Monto mínimo | La ruta solo se calcula si el subtotal supera el monto mínimo; si no, se sugiere agregar productos | Sprint 3: definir en SUP-323 |
| RN-22 | Medio de transporte | El cálculo exige saber cómo viaja el usuario para convertir distancia en dinero | Sprint 3, acotado: vehículo particular, con el rendimiento en la solicitud (SUP-323) |
| RN-25 / RN-26 | Misiones de colaborador | Puntos y tokens de IA por validar precios | Fuera de alcance (crowdsourcing) |

## Políticas de operación

### Patrón anti-alucinación (IA)

1. **Extracción:** la IA convierte el texto del usuario en filtros y entidades.
2. **Filtrado:** el backend consulta Catálogo con esos filtros, por HTTP. Si no
   hay resultados, **corta** y responde un mensaje local, sin gastar otra
   llamada al proveedor.
3. **Redacción:** la IA redacta solo con los datos validados en el paso 2. Nunca
   consulta la base de datos ni inventa precios, stock o tiendas.

### Costo de desplazamiento

- **Vehículo particular:**
  `Costo de desplazamiento = (Distancia ÷ Rendimiento km/L) × Precio del combustible`.
- **Precio del combustible:** se obtiene de la Comisión Nacional de Energía
  (CNE). Para el Sprint 3, Energía Abierta (precio regional semanal) con un
  valor de respaldo configurable si la fuente no responde (SUP-323).
- **Distancia:** en el Sprint 3 se estima con Haversine y un factor de
  corrección declarado, no por calles reales (ver [D11](../Decisiones/11_Motor_de_rutas_en_Go.md)).
- **Transporte público:** basado en tiempos de OpenTripPlanner, excluyendo
  tramos menores al umbral de caminata, con tarifa base configurable. **Fuera
  de alcance del Sprint 3.**

### Sesión y acceso

- Sesión con JWT HS256 en la cookie HttpOnly `jwt`, válida 24 h. No hay refresh
  token (SUP-235 y SUP-236 se cancelaron). La especificación original preveía
  access token de 15 min y refresh de 7 días.
- Cada servicio autoriza sus rutas por rol; el Gateway no valida JWT.

## Cambios respecto de la especificación original

| Tema | Original (v1.3) | Hoy |
| --- | --- | --- |
| Motor de cálculo | OpenTripPlanner + OpenStreetMap + Google OR-Tools | Haversine y prueba de combinaciones con máximo 3 paradas, en Go (D11) |
| Sesión | Access token 15 min + refresh 7 días | JWT de 24 h sin refresh |
| RN-00 | Incluía presupuesto y restricciones dietéticas | Fuera de alcance del Sprint 3 |

Versión formal completa: Drive del equipo e historial de git (antes del PR #89).
