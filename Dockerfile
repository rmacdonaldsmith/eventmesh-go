# syntax=docker/dockerfile:1

FROM golang:1.25.1-alpine AS build

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/eventmesh ./cmd/eventmesh \
	&& CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/eventmesh-cli ./cmd/eventmesh-cli

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
	&& addgroup -S eventmesh \
	&& adduser -S -G eventmesh eventmesh \
	&& mkdir -p /var/lib/eventmesh \
	&& chown -R eventmesh:eventmesh /var/lib/eventmesh

COPY --from=build /out/eventmesh /usr/local/bin/eventmesh
COPY --from=build /out/eventmesh-cli /usr/local/bin/eventmesh-cli
COPY docker/eventmesh-entrypoint.sh /usr/local/bin/eventmesh-entrypoint.sh

RUN chmod +x /usr/local/bin/eventmesh-entrypoint.sh

USER eventmesh
WORKDIR /var/lib/eventmesh

EXPOSE 8081 9090

ENTRYPOINT ["/usr/local/bin/eventmesh-entrypoint.sh"]
