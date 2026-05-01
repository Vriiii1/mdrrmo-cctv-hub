# auth-shim/Dockerfile
# Multi-arch build: BUILDPLATFORM is the builder host's arch (always native, for speed);
# TARGETPLATFORM is what we're building FOR (set by docker buildx --platform=...).
# Go cross-compiles trivially, so we use a native-arch build stage that emits a
# binary for whatever TARGETARCH was requested.
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /out/auth-shim ./

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/auth-shim /auth-shim
USER nonroot:nonroot
ENTRYPOINT ["/auth-shim"]
EXPOSE 7000
