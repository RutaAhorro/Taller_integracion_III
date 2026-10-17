# Dockerfile para el Microservicio de IA Conversacional
FROM python:3.11-slim

# Evitar la creación de archivos .pyc y liberar logs
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

WORKDIR /app

# Instalar curl para healthchecks ligeros (optimo en hardware limitado)
RUN apt-get update && apt-get install -y --no-install-recommends curl \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copiamos el andamiaje del modulo
COPY . .

EXPOSE 8002

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://127.0.0.1:8002/health || exit 1

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8002", "--reload"]
