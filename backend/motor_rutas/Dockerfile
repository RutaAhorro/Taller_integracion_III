# Motor de rutas (FastAPI + OR-Tools): dueño de rutas.* (SUP-240).
FROM python:3.11-slim AS base

# Sin .pyc ni caché de pip: la raíz del contenedor es de sólo lectura en Compose y K8s.
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY app/ ./app/

FROM base AS test
COPY tests/ ./tests/
RUN python -m unittest discover -s tests -t .

FROM base AS runtime
USER 65532:65532
# Puerto 8001 para no chocar con el Gateway en el host.
EXPOSE 8001
HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=3 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8001/_rutas/ready', timeout=2)" || exit 1
# Un solo worker y sin --reload: el pod tiene 512 MiB y OR-Tools resuelve en el mismo proceso.
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8001", "--workers", "1", "--no-access-log"]
