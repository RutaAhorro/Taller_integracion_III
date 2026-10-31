# Guía: Contexto IA para el Equipo

## ¿Qué es esto?

El proyecto incluye archivos de contexto (`GEMINI.md`, `CLAUDE.md`, `copilot-instructions.md`) que les dicen a las IAs (Antigravity, Claude, Copilot) cómo está estructurado el proyecto **sin tener que leer todos los archivos**. Esto reduce el consumo de tokens y mejora la precisión de las respuestas.

## ¿Qué tengo que hacer?

### Paso único (una sola vez después de hacer pull)

**Linux / Mac / Git Bash (Windows):**
```bash
bash scripts/setup-hooks.sh
```
# popin
**Windows (PowerShell o CMD):**
```powershell
git config core.hooksPath scripts/git-hooks
```

Eso es todo. **No hay paso 2.** A partir de ahora, cada vez que hagas `git commit`, los archivos de contexto se actualizan automáticamente e incluyen en tu commit. No tienes que hacer nada extra ni recordar nada.

> **Nota Windows:** El hook se ejecuta con el bash que trae Git para Windows, así que funciona sin importar si usas PowerShell, CMD o Git Bash para tu trabajo diario. Solo necesitas tener Python instalado y accesible desde la terminal (`python` o `python3`).

### ¿Cómo verifico que funciona?

Haz un commit normal como siempre. Si ves este mensaje en la terminal:

```
🧠 Actualizando mapas de contexto IA...
✅ Mapas de contexto IA actualizados e incluidos en el commit.
```

Está funcionando. Si NO ves el mensaje, es porque tu commit no tocó archivos fuente (.py, .go, .ts, .tsx), así que no había nada que actualizar.

## Preguntas frecuentes

### ¿Tengo que editar los GEMINI.md o CLAUDE.md manualmente?

**No.** Los archivos por microservicio (`backend/api/GEMINI.md`, `frontend/web/GEMINI.md`, etc.) se regeneran solos. Solo el **GEMINI.md de la raíz** tiene una sección estática (convenciones, reglas de Git, etc.) que se edita manualmente si cambia alguna convención del proyecto.

### ¿Qué pasa si no ejecuto el setup?

Nada se rompe. Simplemente tus commits no actualizarán los archivos de contexto. Cuando otro compañero que sí tenga el hook configurado haga un commit, los archivos se actualizarán con sus cambios y los tuyos.

### ¿Qué pasa si hago pull y el hook no me funciona?

Vuelve a ejecutar `bash scripts/setup-hooks.sh`. Es idempotente (se puede ejecutar cuantas veces quieras sin problema).

### ¿Para qué sirve cada archivo?

| Archivo | Lo lee... | Ubicación |
|---|---|---|
| `GEMINI.md` | **Antigravity** (Chat + IDE) | Raíz + cada microservicio |
| `CLAUDE.md` | **Claude Code** | Raíz + cada microservicio |
| `.github/copilot-instructions.md` | **GitHub Copilot** (VSCode) | Solo en raíz |

### ¿El hook hace mi commit más lento?

No perceptiblemente. El script tarda menos de 1 segundo en analizar todo el proyecto.

### ¿Qué archivos se analizan?

| Servicio | Lenguaje | Archivos |
|---|---|---|
| `backend/api/` | Go | `.go` |
| `backend/catalogo/` | Go | `.go` |
| `backend/ia_conversacional/` | Python | `.py` |
| `backend/motor_rutas/` | Python | `.py` |
| `backend/scraper/` | Python | `.py` |
| `frontend/web/src/` | TypeScript/React | `.ts`, `.tsx` |

### ¿Y si agregamos un nuevo microservicio?

Editar `scripts/gen_service_map.py`, buscar el diccionario `SERVICES` al inicio del archivo y agregar la entrada nueva. Ejemplo:

```python
SERVICES = {
    # ... servicios existentes ...
    "backend/nuevo_servicio": {
        "lang": "python",
        "name": "Nuevo Servicio (FastAPI)",
        "extensions": [".py"],
        "exclude": ["__pycache__"],
    },
}
```
