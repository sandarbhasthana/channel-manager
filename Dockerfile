FROM golang:1.26.2-alpine AS builder

RUN apk add --no-cache ca-certificates git
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /out/channel-manager ./apps/api
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /out/channel-manager-migrate ./platform/db/cmd/migrate

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app
WORKDIR /app
COPY --from=builder /out/channel-manager /usr/local/bin/channel-manager
COPY --from=builder /out/channel-manager-migrate /usr/local/bin/channel-manager-migrate
COPY migrations ./migrations
USER app
EXPOSE 8080
CMD ["/usr/local/bin/channel-manager"]
