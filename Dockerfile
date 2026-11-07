FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -o /gateway ./cmd/inferenceshield && CGO_ENABLED=0 go build -o /mock ./cmd/mock-backend

FROM alpine:3.21 AS gateway
RUN addgroup -g 10001 -S app && adduser -u 10001 -S -G app app
COPY --from=builder /gateway /gateway
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/gateway"]

FROM alpine:3.21 AS mock
RUN addgroup -g 10001 -S app && adduser -u 10001 -S -G app app
COPY --from=builder /mock /mock
USER 10001:10001
EXPOSE 8081
ENTRYPOINT ["/mock"]
