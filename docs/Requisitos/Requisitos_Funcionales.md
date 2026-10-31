# ⚙️ Requisitos Funcionales (El MVP)

Este documento detalla qué debe poder hacer el sistema. Está dividido en dos etapas clave para el desarrollo.

---

## 🚀 Etapa 1: Autenticación, Catálogo y IA (Sprints 1 y 2)

### Para el Usuario (Invitado y Registrado)
* **Explorar el Catálogo:** Buscar y filtrar productos. El Backend NO debe enviar productos expirados.
* **Autenticación Completa:** Registro, Login, y Recuperación de Contraseña.
* **Perfil Dietético:** El usuario puede guardar sus restricciones (ej. celíaco, vegano). La IA y las búsquedas deben respetar esto.
* **Listas de Compra:** Crear, editar y guardar listas. Si un producto se queda sin stock en la BD, la interfaz debe avisar y el backend bloquearlo del cálculo.
* **Asistente IA (Recetas):** Pedir recetas basadas en preferencias. El Backend debe validar qué ingredientes hay en stock antes de que la IA responda.

### Para el Colaborador (Crowdsourcing)
* **Misiones de Validación:** Poder reportar precios o faltas de stock directamente desde la app.
* **Sistema de Recompensas:** Un dashboard donde ven sus misiones completadas y sus "Tokens IA" ganados por aportar.

### Para el Admin
* **Control del Scraper:** Un botón o endpoint manual para forzar la ejecución del Web Scraping.

---

## 🗺️ Etapa 2: Motor de Optimización y Mapas (Sprints 3 y 4)

### Para el Usuario Registrado
* **Configurar Traslado:** Seleccionar si va en auto (y su rendimiento km/L) o transporte público. Requisito estricto para calcular dinero.
* **Definir Ubicación:** Ingresar el punto de origen.
* **Cálculo de Ruta Óptima:** El sistema cruza las listas de compra con el mapa para decir a qué supermercados ir. Se bloquea si el ahorro es mínimo.
* **Resultados en Mapa:** Ver el trazado de la ruta sugerida, horarios de apertura de la tienda y el desglose de productos a comprar en cada parada.

### Para el Admin
* **Homologación de Catálogo:** Herramienta para decirle a la Base de Datos que "Leche Soprole 1L" del Jumbo es el mismo producto que "Leche Soprole 1L" del Lider.
* **Monitoreo:** Dashboard con tasa de éxito del scraper, consumo de API de IA y usuarios activos. Gestión de bloqueo de cuentas.
