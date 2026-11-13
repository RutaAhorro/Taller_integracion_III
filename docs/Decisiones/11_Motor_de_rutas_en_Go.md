# D11 — Motor de rutas en Go en vez de Python

[← Índice de decisiones](README.md)

**Contexto:** el motor de rutas está en Python (FastAPI) y solo tiene el CRUD de listas (5 endpoints). El "algoritmo" actual es un prototipo ficticio y una estrategia con OR-Tools sin implementar. El algoritmo real (SUP-313 y SUP-328) todavía no existe. El resto del backend, salvo la IA, está en Go.

**Decisión:** reescribir el servicio en Go conservando el contrato HTTP y las tablas `rutas.*` (SUP-348), antes de programar los artículos y el algoritmo. Acordada con Vicho el 05-11-2026.

**Motivo:**
- Reescribir ahora cuesta poco: solo el CRUD. Después de los artículos y el algoritmo costaría mucho más.
- La ventaja de Python era OR-Tools, que no hace falta: con el contrato acotado de SUP-323 (máximo 3 paradas entre 10 sucursales) basta con probar todas las combinaciones.
- Distancia (Haversine), costo de bencina y sumas no dependen del lenguaje. Los mapas se dibujan en el frontend con Leaflet.
- Los dos responsables trabajan en Go, y el servicio queda con la misma estructura, CI y Dockerfile que Catálogo e Identidad.

**Alternativa descartada:** mantener Python y solo hacer el barrido. Evita la reescritura, pero deja un segundo lenguaje en un servicio sin una razón técnica que lo justifique.

**Consecuencias:**
- La IA conversacional queda como único servicio en Python, por la madurez de sus SDK de LLM.
- Si en el futuro se permiten muchas paradas, el orden de visita necesitará una heurística (vecino más cercano, 2-opt) en lugar de OR-Tools.
- Tope de dos días para la migración; si se complica, se vuelve al servicio en Python.

**Estado:** aceptada e implementada (SUP-348). El servicio Python se eliminó; queda en el historial de git.
