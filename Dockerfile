FROM golang:1.22-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
COPY vendor ./vendor
COPY common ./common
COPY hoster ./hoster
COPY slackermost ./slackermost
COPY cmd ./cmd
COPY main.go ./
COPY reviewers.json ./reviewers.json

RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -trimpath -ldflags="-s -w" -o /out/kc-review-bot . \
 && CGO_ENABLED=0 GOOS=linux go build -mod=vendor -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

COPY --from=build /out/kc-review-bot /app/kc-review-bot
COPY --from=build /out/server /app/server
COPY --from=build /src/reviewers.json /app/reviewers.json

ENV BOT_BIN=/app/kc-review-bot \
    REVIEWERS_PATH=/app/reviewers.json

EXPOSE 8080

ENTRYPOINT ["/app/server"]
