# 💰 Modelo de Monetización y Lógica de Negocio

Este documento explica cómo RutaAhorro planea generar ingresos. Está pensado para que el equipo de desarrollo entienda qué funcionalidades aplican a cada tipo de usuario y cómo esto impacta la arquitectura (bases de datos, roles, y componentes del frontend).

> **Regla de Oro (Independencia del algoritmo)**
> Ningún pago comercial puede alterar el algoritmo de búsqueda. Si un producto es el más barato, debe salir primero de forma orgánica. Los espacios comerciales o publicitarios deben estar estrictamente separados visualmente (ej. un banner de "Patrocinado").

---

## 1. Sistema de Usuarios (Free vs Premium)

La plataforma funcionará bajo un modelo *Freemium*. La herramienta principal de comparación siempre será gratuita.

### 🟢 Usuario Gratuito (Free)
Funciones que deben estar disponibles por defecto (sin validación de pago):
- Comparación de precios base entre supermercados.
- Búsqueda y filtrado normal del catálogo de productos.
- Cálculo del costo total (precio del producto + costo de desplazamiento).
- Visualización de la ruta de compra recomendada básica.

### 🟡 Usuario Premium (Suscripción)
Funciones que requerirán validación de un rol `premium` (ej. vía JWT o validación en la base de datos):
- **Mapas sin límites:** Visualización ilimitada de múltiples combinaciones de rutas en el mapa.
- **IA Avanzada:** Interacción extendida con el asistente virtual (sin restricciones de tokens o consultas).
- **Historial de Precios:** Acceso a gráficos de evolución de un producto (precio mínimo, máximo y promedio histórico).
- **Integración de Tarjetas:** Posibilidad de asociar tarjetas de fidelidad en el perfil del usuario para que el motor calcule el precio final con el descuento exclusivo ya aplicado.

---

## 2. Otras Vías de Ingresos (Impacto Técnico)

Además de los usuarios Premium, existen otras estrategias que afectarán cómo desarrollamos el sistema:

### 📊 Análisis Estadístico (B2B)
A futuro, se generarán reportes de mercado para empresas (ej. "productos más buscados" o "diferencias de precios en la zona").
* **Impacto técnico:** Toda la recolección de métricas de búsqueda debe diseñarse de forma **agregada y anónima**. Es vital asegurar que no se expongan datos personales de los usuarios.

### 🔗 Enlaces de Afiliado (Redirección)
* **Impacto técnico:** Los botones de "Ir al supermercado" en el Frontend deberán soportar la inyección dinámica de parámetros (UTMs o tokens de afiliación) en la URL, para que RutaAhorro pueda reclamar una comisión si el usuario concreta la compra.

### 🏷️ Subastas de Espacios (Ads)
* **Impacto técnico:** Se contempla un modelo (subasta de segundo precio) donde las marcas paguen por aparecer. Esto requerirá endpoints específicos en el backend para entregar "Productos Patrocinados" y componentes aislados en React para mostrarlos sin contaminar los resultados reales.

---
*Nota: Estas vías de monetización son preliminares y deben ser validadas eventualmente con la DIRITT de la UCT.*
