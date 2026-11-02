# Contexto breve para asistentes IA

Los contextos orientan a las IA hacia el código relevante, no reemplazan los
README ni acreditan funcionalidades desplegadas.

## Archivos y fuente de verdad

- `CLAUDE.md` de la raíz: arquitectura y reglas comunes.
- `CLAUDE.md` en cada servicio y en `frontend/web`: responsabilidades, límites,
  puntos de entrada y pruebas. Leer solo el del área afectada.
- `GEMINI.md`: copia sincronizada del `CLAUDE.md` correspondiente.
- `.github/copilot-instructions.md`: copia del contexto raíz para GitHub Copilot.

Para cambiar instrucciones manuales, **editar `CLAUDE.md`**, por encima del
marcador `ARCHITECTURE:AUTO-GENERATED`. Luego, desde la raíz:

```bash
python3 scripts/gen_service_map.py
```

Para regenerar solo un servicio y su copia Gemini:

```bash
python3 scripts/gen_service_map.py backend/control_scraping
```

El generador conserva el texto manual y reemplaza la sección automática por un
conteo de archivos agrupados por directorio. No enumera funciones, tipos ni tests;
el contexto raíz solo enlaza los servicios. Los símbolos se buscan en el código
cuando se necesitan. No existe un contexto operativo del antiguo `backend/api`.

## Hook opcional

Requiere Python 3. Desde la raíz:

```bash
bash scripts/setup-hooks.sh
```

En Windows con Git Bash también puede configurarse mediante
`git config core.hooksPath scripts/git-hooks`.

El hook regenera los contextos al detectar cambios fuente en los servicios
actuales o en el generador y los añade al commit. **Revisar el diff de estos
archivos antes de confirmar el commit**; activar el hook no autoriza a una IA a
hacer commits ni a desplegar. Si falta Python, el hook avisa y omite la generación.
Cambiar solo texto manual requiere ejecutar el generador explícitamente.

## Qué mantener fuera del contexto

- Inventarios de todos los símbolos, resultados extensos de pruebas o logs.
- Contraseñas, cookies, tokens y contenido de `.env`.
- Fechas/estados del backlog que pueden cambiar sin modificar el código.
- Afirmaciones de despliegue derivadas únicamente de manifiestos.

La documentación histórica se consulta desde [el índice](../README.md). Si cambia
una responsabilidad o contrato, actualizar también el README del servicio y su
contexto manual, no solo el conteo generado.
