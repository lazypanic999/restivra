FROM node:24-bookworm-slim AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.27.4-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/webui/dist ./internal/webui/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /restivra ./cmd/restivra

FROM docker:29-cli AS dockercli
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/* && useradd -u 10001 -m restivra
COPY --from=dockercli /usr/local/bin/docker /usr/local/bin/docker
COPY --from=build /restivra /usr/local/bin/restivra
COPY LICENSE THIRD_PARTY_NOTICES.md /usr/share/doc/restivra/
WORKDIR /app
USER restivra
ENTRYPOINT ["restivra"]
CMD ["serve"]
