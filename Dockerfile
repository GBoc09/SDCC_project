FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/registry \
    .

FROM alpine:3.21

RUN addgroup -S registry \
    && adduser -S registry -G registry

WORKDIR /app

COPY --from=builder /out/registry /app/registry

RUN mkdir -p /data \
    && chown registry:registry /data
USER registry

EXPOSE 8080

ENTRYPOINT ["/app/registry"]