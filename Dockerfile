# syntax=docker/dockerfile:1.7

FROM golang:1.27.0-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN --mount=type=secret,id=git_token,required=true \
    git config --global url."https://x-access-token:$(cat /run/secrets/git_token)@github.com/".insteadOf "https://github.com/" \
    && GOPRIVATE=github.com/SkyvisorInsights/* go mod download \
    && git config --global --unset-all url."https://x-access-token:$(cat /run/secrets/git_token)@github.com/".insteadOf
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -trimpath -ldflags="-s -w" -o /out/skyvisor-mcp ./cmd/skyvisor-mcp

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/skyvisor-mcp /usr/local/bin/skyvisor-mcp
USER nonroot:nonroot
EXPOSE 8087
ENTRYPOINT ["/usr/local/bin/skyvisor-mcp"]
