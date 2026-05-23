# Two-target build matching the dvystrcil/mcp-server-docker pattern.
#
#   prod   (default)  →  FROM alpine, with ca-certificates only
#   debug             →  same base, room for future incident-time
#                        additions (curl, dig, etc.) without bloating prod
#
# Unlike mcp-server-docker which dispatches bash scripts (and therefore
# bundles bash + kubectl + jq + curl in the prod image), rpg-dice-mcp's
# tools are statically-compiled Go logic — there's no script dispatch.
# Prod only needs ca-certificates (so the streamable HTTP server's
# clients can verify the cluster CA on outbound calls, if any future
# tool ends up making them).

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux \
    go build -ldflags='-s -w' -trimpath \
    -o /out/rpg-dice-mcp ./cmd/rpg-dice-mcp

FROM alpine:3 AS prod
RUN apk add --no-cache ca-certificates && \
    addgroup -S app && adduser -S -G app -u 10001 app
COPY --from=build /out/rpg-dice-mcp /usr/local/bin/rpg-dice-mcp
USER app
EXPOSE 8080
ENV HTTP_ADDR=:8080
ENTRYPOINT ["/usr/local/bin/rpg-dice-mcp"]

FROM prod AS debug
USER root
RUN apk add --no-cache curl bash
USER app
