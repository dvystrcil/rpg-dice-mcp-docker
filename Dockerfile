# Two-target build.
#
#   prod   (default)  →  FROM scratch — static Go binary only.
#   debug             →  FROM alpine — adds curl/bash for incident triage.
#
# Why scratch is correct here (and differs from mcp-server-docker):
#   - mcp-server-docker dispatches bash scripts and bundles bash +
#     kubectl + jq + curl in prod. It needs an OS in the image.
#   - rpg-dice-mcp is pure Go logic. CGO_ENABLED=0 statically compiles
#     the binary; there is no script dispatch, no outbound HTTP, no
#     CA verification needed. A scratch image is sufficient and
#     smaller (~5 MB final vs ~15 MB on alpine).
#
# We DO need a /etc/passwd entry so USER directive can resolve a
# non-root user. The build stage creates a minimal passwd-and-group
# pair which the prod stage copies in.

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux \
    go build -ldflags='-s -w' -trimpath \
    -o /out/rpg-dice-mcp ./cmd/rpg-dice-mcp
# Build a minimal passwd/group file with a non-root user so the
# scratch image can USER-switch. uid 10001 matches the deploy
# manifest's runAsUser.
RUN echo 'app:x:10001:10001::/:/sbin/nologin' > /out/passwd && \
    echo 'app:x:10001:' > /out/group

FROM scratch AS prod
COPY --from=build /out/passwd /etc/passwd
COPY --from=build /out/group /etc/group
COPY --from=build /out/rpg-dice-mcp /usr/local/bin/rpg-dice-mcp
USER app
EXPOSE 8080
ENV HTTP_ADDR=:8080
ENTRYPOINT ["/usr/local/bin/rpg-dice-mcp"]

# Debug target — adds shell + curl for incident-time triage.
# Built on alpine because scratch obviously has no apk; this is a
# deliberate departure from prod's base for diagnostic purposes only.
FROM alpine:3 AS debug
RUN apk add --no-cache ca-certificates curl bash && \
    addgroup -S app && adduser -S -G app -u 10001 app
COPY --from=build /out/rpg-dice-mcp /usr/local/bin/rpg-dice-mcp
USER app
EXPOSE 8080
ENV HTTP_ADDR=:8080
ENTRYPOINT ["/usr/local/bin/rpg-dice-mcp"]
