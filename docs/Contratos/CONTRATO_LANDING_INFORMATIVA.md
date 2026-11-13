# Contrato de la landing informativa

> **Estado:** excepción propuesta para ratificación del equipo en el PR #141 · SUP-354
> **Responsable:** Daniela (frontend)

## Para qué existe

Este contrato documenta por qué se conserva
`frontend/web/src/data/landing.json` y limita su uso. Es la referencia para el
equipo y los asistentes de IA al revisar o modificar la portada.

La autora recuperó la landing tras las pruebas manuales para explicar la
propuesta del producto antes de ingresar a la aplicación. Los textos y ejemplos
estables permiten hacerlo sin depender de la disponibilidad del backend. El
JSON reúne ese contenido editorial en un solo archivo, separado de los clientes
HTTP y de la lógica funcional.

## 1. Excepción al barrido

Se conserva `landing.json` como contenido informativo de la portada pública
(`/`). Puede contener textos, navegación y ejemplos ilustrativos de productos,
precios, compras, distancias y ahorro.

Es una excepción acotada a la prohibición de datos de ejemplo en `src/` y de
archivos de datos sueltos de la
[guía del barrido](../Guias/GUIA_BARRIDO_CODIGO.md). No modifica los contratos
HTTP ni autoriza mocks en las vistas funcionales.

## 2. Límites de uso

- Solo lo consumen los componentes informativos de la portada, en
  `frontend/web/src/components/landing/`.
- No alimenta catálogo, carrito, listas, chat, rutas, administración ni
  registros de la base de datos.
- No reemplaza respuestas de la API ni sirve de respaldo cuando un servicio
  falla. Las vistas funcionales deben usar datos reales o estados explícitos
  de vacío, error o no disponibilidad.
- Los precios, distancias, métricas y ahorros son ilustrativos. No acreditan
  datos actuales, mediciones reales ni resultados calculados para el usuario.
  La interfaz debe identificarlos como tales.
- Conservar el archivo no autoriza afirmar que los ejemplos se verifican en
  tiempo real ni presentar funcionalidades pendientes como implementadas.

## 3. Reglas para asistentes de IA y revisiones

1. Leer este contrato antes de proponer retirar `landing.json` por contener
   ejemplos. Su conservación es intencional; no debe eliminarse ni conectarse
   automáticamente a una API como parte de una limpieza.
2. Distinguir contenido editorial de datos funcionales. Esta excepción no se
   extiende a otros archivos o pantallas.
3. Si se detecta un texto que contradice el carácter ilustrativo o el alcance
   implementado, señalarlo de forma concreta. El contrato no certifica que
   todos los textos actuales ya cumplan estas condiciones.
4. Cualquier uso fuera de la portada o cambio de estos límites requiere
   revisión del equipo y actualización de este contrato en el mismo PR.

## 4. Comprobación de la separación

Al revisar cambios, comprobar que los imports de `landing.json` permanecen en
los componentes de la portada y que ninguna vista funcional usa sus ejemplos.
No basta con que el archivo esté en una carpeta llamada `data`.

La ratificación de esta excepción se realiza en el PR #141. Este documento no
afirma que el equipo o el profesor ya la hayan aprobado.
