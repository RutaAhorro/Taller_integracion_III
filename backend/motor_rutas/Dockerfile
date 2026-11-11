# Motor de rutas en Go (SUP-348, D11): dueño de rutas.*.
FROM golang:1.26 AS source
WORKDIR /src
ENV GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN --mount=type=secret,id=network_ca,target=/run/secrets/network_ca \
    if [ -f /run/secrets/network_ca ]; then export SSL_CERT_FILE=/run/secrets/network_ca; fi; \
    go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Las pruebas de integración se omiten sin RUTAS_TEST_DB_URL; aquí corren las unitarias.
FROM source AS test
COPY tests/ ./tests/
RUN go test -race ./... && go vet ./...

FROM source AS build
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/motor_rutas ./cmd/motor_rutas

FROM alpine:3.23 AS runtime
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/motor_rutas /usr/local/bin/motor_rutas
# Puerto 8001 para no chocar con el Gateway en el host; el Gateway ya apunta aquí.
ENV PORT=8001
USER 65532:65532
EXPOSE 8001
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -q --spider http://127.0.0.1:8001/_rutas/ready || exit 1
ENTRYPOINT ["/usr/local/bin/motor_rutas"]
