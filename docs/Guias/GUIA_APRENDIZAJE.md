# Análisis de Requisitos y Expectativas - Taller de Integración III (INFO1166)

**Docente:** Dr. Julio Rojas Mora
**Semestre:** 6to Semestre / Año 2026

## 1. Visión General del Curso y Evaluación

El curso exige la construcción de un proyecto informático completo aplicando metodologías formales y ágiles (Scrum, Modelo Entidad-Relación, UML y Git/GitHub). Se espera que los alumnos integren competencias adquiridas a lo largo de la carrera en un contexto real.

**Ponderación de Evaluaciones Oficiales:**

* **Presentación Perfil (20%):** Semana 4 *(Completada)*
* **Presentación Sprint 1 (20%):** Semana 8 *(Completada)*
* **Presentación Sprint 2 (20%):** Semana 12 *(Próxima - Finales de Octubre)*
* **Presentación Final (40%):** Semana 16

**Políticas Críticas del Docente:**

* **Evaluación Aleatoria (Riesgo Alto):** Para las evaluaciones, *se seleccionará aleatoriamente a un miembro del grupo y la nota obtenida por este será asignada a todos los integrantes*. Esto obliga a que tanto el equipo de Frontend como de Backend dominen la totalidad del proyecto.
* **Resolución de Conflictos:** El compromiso ético grupal obliga a informar si algún integrante no participa efectivamente, iniciando un proceso formal de resolución.

## 2. Evaluación de la Arquitectura de Microservicios y Orquestación

El sistema debe operar de forma modular, resiliente y escalable:

* **Entornos de Producción:** Para las presentaciones y pruebas de los Sprints restantes, el proyecto debe desplegarse en el clúster de Kubernetes del Datacenter de la universidad (`student-<usuario>`).
* **Gestión de Recursos:** El sistema evaluará que los microservicios respeten la asignación limitada de CPU y memoria de la universidad. El control de recursos (como evitar que un proceso de *scraping* bote los contenedores) es vital.

## 3. Sistematización del Proyecto: "RutaAhorro"

El equipo se organiza en dos sub-equipos paralelos (3 integrantes en Backend, 2 en Frontend), trabajando 5 horas semanales cada uno bajo metodología Scrum.

### 3.1. Requerimientos Funcionales (RF) Críticos para Sprints Finales

* **RF-03.3 (Optimizar ruta de compra):** Minimizar el costo sugiriendo alternativa en tienda única o varias, sujeto a reglas de negocio de monto mínimo y ahorro exigido (RN-00, RN-07, RN-08, RN-14, RN-20).
* **RF-04.2 (Homologar catálogo):** Definir igualdad entre productos de distintas fuentes.
* **RF-04.3 y RF-04.4 (Monitoreo y Gestión):** El "Super Admin" debe poder monitorear la tasa de éxito del scraping, uso de IA y gestionar el estado de las cuentas (heredando permisos según RN-11).

### 3.2. Requerimientos No Funcionales (RNF) y Estabilidad

* **RNF-01 (Concurrencia):** El scraping debe ser concurrente para no bloquear la plataforma.
* **RNF-02 y RNF-03 (Tolerancia a fallos):** El sistema debe conservar información vigente ante caídas temporales durante el scraping y actualizarse sin interrumpir el servicio.
* **RNF-05 (Limitación de IA):** La IA solo puede normalizar semánticamente, prohibiendo estrictamente que infiera o altere precios.
* **RNF-08 (Privacidad Geográfica):** Está prohibido persistir la ubicación exacta del usuario una vez calculado el trayecto.

---

## 4. Mapa de Referencia SWEBOK v4 para "RutaAhorro"

Para asegurar la calidad técnica y proveer argumentos sólidos durante las interrogaciones aleatorias, el equipo debe respaldar sus decisiones en las siguientes Áreas de Conocimiento (KAs) del *SWEBOK v4*. *(Nota: Se han excluido los capítulos puramente teóricos o de mantenimiento a largo plazo para enfocar los esfuerzos en la construcción y despliegue).*

### 4.1. Arquitectura, Infraestructura y Operaciones (El Core del Proyecto)
* **Capítulo 2: Software Architecture:** Fundamental para justificar por qué se separó el Frontend, Backend, IA y Scraper en microservicios distintos. Ayuda a defender las decisiones de diseño para mantener la *resiliencia bajo restricciones* de recursos.
* **Capítulo 6: Software Engineering Operations:** **CRÍTICO.** Este capítulo justifica todo el uso del clúster de **Kubernetes**, el despliegue mediante contenedores y la gestión de la infraestructura como código (IaC). Brinda la teoría para explicar cómo monitorean el rendimiento (`student-<usuario>`) y gestionan los despliegues sin interrupciones (DevOps).
* **Capítulo 13: Software Security:** Obligatorio para el Sprint 2. Fundamenta la implementación de tokens de autenticación, el cifrado de comunicaciones (RNF-11) y la protección contra ataques de fuerza bruta en el login (RNF-12).

### 4.2. Construcción, Calidad y Pruebas
* **Capítulo 4: Software Construction:** Proporciona las buenas prácticas para el diseño de la API (API Design), el manejo de errores (Error Handling) y la programación defensiva, garantizando que si el *scraper* falla, la aplicación no colapse (RNF-02 y RNF-03).
* **Capítulo 5: Software Testing:** Base teórica para el Sprint 4. Justifica los niveles de prueba requeridos por la guía: pruebas funcionales (verificar la homologación de catálogo), pruebas de integración (Frontend comunicándose con el motor lógico) y especialmente las pruebas de carga sobre Kubernetes.
* **Capítulo 8: Software Configuration Management:** Respalda la política de uso de Git y GitHub del equipo para el control de versiones, integración de código de 6 desarrolladores y el empaquetado seguro antes de enviar a Kubernetes.

### 4.3. Gestión Ágil y Requisitos
* **Capítulo 1: Software Requirements:** Justifica cómo han ido priorizando iterativamente los requerimientos (RF y RNF) y las Reglas de Negocio (RN) Sprint a Sprint, alineado a la metodología Scrum.
* **Capítulo 9: Software Engineering Management:** Provee el marco de gestión de proyectos, vital para argumentar cómo el equipo planifica los Sprints, estima el esfuerzo (5 horas semanales por integrante) y gestiona los riesgos de desarrollo.

### 4.4. Defensa del Negocio y Ámbito Legal (Interrogación del Profesor)
* **Capítulo 15: Software Engineering Economics:** Permite justificar las decisiones de arquitectura evaluando el "Trade-Off" técnico-económico. Ejemplo clave: Explicar cómo la **RN-14 (Capacidad de procesamiento)** actúa como un mecanismo de contención económica para no desbordar los costos de cómputo del Datacenter.
* **Capítulo 14: Software Engineering Professional Practice:** Base para definir el marco legal, la ética profesional y los Términos y Condiciones. Esencial para justificar el cumplimiento del **RNF-08** (Privacidad Geográfica) bajo normativas internacionales de protección de datos de los usuarios.