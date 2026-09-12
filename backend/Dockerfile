FROM golang:1.27.0-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api

FROM alpine:3.22

RUN addgroup -S app && adduser -S -G app -u 10001 app
WORKDIR /app
COPY --from=build /out/api /app/api

USER app
EXPOSE 8081

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8081/api/v1/health || exit 1

ENTRYPOINT ["/app/api"]
