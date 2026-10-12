FROM golang:1.26 AS source
WORKDIR /src
ENV GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN --mount=type=secret,id=network_ca,target=/run/secrets/network_ca \
    if [ -f /run/secrets/network_ca ]; then export SSL_CERT_FILE=/run/secrets/network_ca; fi; \
    go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY docs/ ./docs/
COPY tests/ ./tests/

FROM source AS test
RUN go test -race ./... && go vet ./...

FROM source AS build
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/control-scraping ./cmd/control_scraping

FROM alpine:3.23 AS runtime
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --chmod=755 --from=build /out/control-scraping /usr/local/bin/control-scraping
ENV PORT=8080 GIN_MODE=release
USER 65532:65532
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -q --spider -T 2 http://127.0.0.1:8080/_scraping/ready || exit 1
ENTRYPOINT ["/usr/local/bin/control-scraping"]
