# syntax=docker/dockerfile:1

ARG GO_VERSION=1.25.11

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/miwkey-extension ./app

FROM gcr.io/distroless/base-debian12:nonroot

COPY --from=build /out/miwkey-extension /miwkey-extension

EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/miwkey-extension"]
