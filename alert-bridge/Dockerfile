# syntax=docker/dockerfile:1.7
# alert-bridge : binaire statique, image distroless non root (aucune dépendance externe).
FROM --platform=$BUILDPLATFORM golang:1.27.4-trixie AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN go vet ./... && go test ./... \
 && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /alert-bridge .

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /alert-bridge /alert-bridge
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/alert-bridge"]
