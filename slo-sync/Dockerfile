# syntax=docker/dockerfile:1.7
# slo-sync : binaire statique, image distroless non root.
FROM --platform=$BUILDPLATFORM golang:1.27.4-trixie AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY *.go ./
RUN go vet ./... && go test ./... \
 && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /slo-sync .

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /slo-sync /slo-sync
USER 65532:65532
ENTRYPOINT ["/slo-sync"]
