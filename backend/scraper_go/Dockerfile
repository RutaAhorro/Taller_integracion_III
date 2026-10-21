# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build

WORKDIR /src
COPY backend/scraper_go/go.mod backend/scraper_go/go.sum ./
RUN go mod download
COPY backend/scraper_go/ ./
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/scraper-worker \
    ./cmd/worker

FROM alpine:3.22

# Igual que el worker de Python, se ejecuta como root para que add_url pueda
# escribir el bind mount de research. En compose se agrega
# no-new-privileges; cuando research sea de sólo lectura se puede pasar a un
# usuario sin privilegios.
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/scraper-worker /app/scraper-worker
# Hasta el cutover, las categorías viven en el árbol de Python para no
# duplicar la fuente de verdad. En compose se montan en /app/research.
COPY backend/scraper/research/*_categories.txt /app/research/
RUN mkdir -p /var/lib/scraper

ENV SCRAPER_STATE_DIR=/var/lib/scraper \
    SCRAPER_CATEGORY_DIR=/app/research

ENTRYPOINT ["/app/scraper-worker"]
