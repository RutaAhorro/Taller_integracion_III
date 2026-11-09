# Etapa de compilación
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Descargar dependencias primero (cache)
COPY go.mod go.sum ./
RUN go mod download

# Copiar el código fuente
COPY . .

# Compilar binario optimizado
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o bot_discord .

# Etapa final (Imagen enana)
FROM alpine:3.19

WORKDIR /app

# Certificados de seguridad para llamadas HTTPS a Discord
RUN apk --no-cache add ca-certificates

# Traer el binario compilado
COPY --from=builder /app/bot_discord .

# Usuario no-root por seguridad
RUN adduser -D botuser
USER botuser

CMD ["./bot_discord"]
