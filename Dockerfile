# syntax=docker/dockerfile:1
# Bump this tag if your go.mod requires a different toolchain.
FROM golang:1.26-alpine AS build
WORKDIR /src
# Dependencies are vendored, so the build needs no network and no go mod download.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -trimpath -ldflags "-s -w" -o /out/nworm .

FROM alpine:3.20
RUN addgroup -S nworm && adduser -S -G nworm -h /home/nworm nworm
COPY --from=build /out/nworm /usr/local/bin/nworm
USER nworm
WORKDIR /home/nworm
# The relay listens here; the agent only dials out.
EXPOSE 443
ENTRYPOINT ["/usr/local/bin/nworm"]
CMD ["relay", "-config", "/etc/nworm/relay.yaml"]
