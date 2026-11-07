# Diagramas

Cada diagrama indica si describe el sistema actual o un diseño objetivo. Un
diagrama no es evidencia de que algo esté implementado: para eso valen el
código, el SQL y la configuración.

| Diagrama | Estado | Nota |
| --- | --- | --- |
| [Arquitectura de software, Sprint 2](Diagrama_Arquitectura_de_Software_Sprint2.drawio.png) | **Vigente** | Coincide con el código. No muestra Motor de rutas ni IA, que son opcionales ([D10](../Decisiones/10_Componentes_opcionales.md)) |
| [Modelo entidad-relación](Diagrama%20Mer%20-%20Sprint%20I.png) | **Vigente con un cambio pendiente** | Los esquemas coinciden con `infrastructure/db/`. `articulos_lista_compras` dejará de apuntar a `productos_normalizados` ([D12](../Decisiones/12_Identidad_del_producto_en_listas.md), SUP-327): regenerar después |
| [Flujo de usuarios](Diagrama_flujo_usuarios.drawio.png) | Vigente (01-11-2026) | |
| [Casos de uso v2](diagrama_casos_de_uso_v2.drawio.png) | Vigente (01-11-2026) | Incluye casos fuera de alcance; ver el estado de cada uno en [requisitos](../Requisitos/Requisitos_Funcionales.md) |
| [Optimización de ruta de compra](Diagrama_flujo_optim_rutas.drawio.png) | **Diseño objetivo** | Incluye límites por plan premium, horarios, transporte público, tarjetas e historial, que no entran al Sprint 3. El alcance real está en SUP-323 y [D11](../Decisiones/11_Motor_de_rutas_en_Go.md) |

## Diagramas de secuencia

Los flujos actuales (login, consulta del catálogo, ciclo de scraping y logout)
están en [Arquitectura — Flujo de una petición](../Arquitectura/03_Flujo_de_una_peticion.md),
escritos en Mermaid para que GitHub los dibuje y se actualicen junto al texto.

Los diagramas de secuencia en PlantUML se retiraron: describían un Gateway en
FastAPI que consultaba la base de datos y endpoints que no existen. Quedan en el
historial de git.

## Al agregar o cambiar un diagrama

- Preferir Mermaid dentro del documento que lo explica: se versiona como texto y
  se revisa en el PR.
- Si es una imagen, guardar también el archivo fuente (`.drawio`) e indicar aquí
  su estado.
