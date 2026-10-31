# Diagramas de Secuencia del Proyecto (UML / Draw.io)

Esta carpeta contiene los archivos de Diagramas de Secuencia en formato **PlantUML** (`.puml`), estructurados a partir del Diagrama de Casos de Uso y los Requerimientos Funcionales de la plataforma.

---

## 📁 Archivos Disponibles

1. **[`secuencia_invitado.puml`](secuencia_invitado.puml)**
   - **Actor:** Usuario Invitado (No Registrado).
   - **Casos de Uso:** Búsqueda y filtrado de catálogo (`ucCatalogo`), Registro con ubicación y preferencias dietéticas (`ucRegistro`), Recuperación de contraseña (`ucRecuperacion`).

2. **[`secuencia_registrado.puml`](secuencia_registrado.puml)**
   - **Actor:** Usuario Registrado.
   - **Casos de Uso:** Login y carga de perfil/carrito (`ucLogin`, `ucPerfil`), Asistente IA para recetas y reemplazo de productos (`ucIA`, `ucReemplazo`), Optimización geoespacial de rutas de compra y medio de transporte (`ucRuta`, `ucHorarios`).

3. **[`secuencia_superadmin.puml`](secuencia_superadmin.puml)**
   - **Actor:** Super Admin.
   - **Casos de Uso:** Autenticación administrativa (`ucAuthAdmin`), Gestión y ejecución de Web Scraping (`ucScraping`, `ucFuentes`), Gestión de usuarios y suscripciones (`ucUsuarios`), Definición de equivalencias semánticas de productos (`ucEquivalencias`), Dashboard de Monitoreo del sistema (`ucMonitoreo`).

---

## 🚀 Instrucciones para Importar en Draw.io (app.diagrams.net)

1. Abre **[Draw.io](https://app.diagrams.net)** en tu navegador o la aplicación de escritorio.
2. Abre cualquiera de los archivos `.puml` anteriores y copia todo su contenido.
3. En el menú superior de Draw.io, dirígete a:
   `Organizar` (Arrange) ➔ `Insertar` (Insert) ➔ `Avanzado` (Advanced) ➔ `PlantUML...`
4. Pega el código del archivo `.puml` en el cuadro de texto.
5. Haz clic en **Insertar**.
6. Draw.io generará instantáneamente el diagrama de secuencia visual y totalmente editable.
