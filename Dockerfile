#syntax=docker/dockerfile:1.2
FROM --platform=$BUILDPLATFORM golang:1.21.3 as builder
WORKDIR /usr/src/app
ADD . .
ENV GOTRACEBACK=all
ARG TARGETARCH
ARG SKAFFOLD_GO_GCFLAGS
RUN --mount=type=cache,id=gomod,target=/go/pkg/mod \
    --mount=type=cache,id=gobuild,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOARCH=$TARGETARCH go build -gcflags="${SKAFFOLD_GO_GCFLAGS}" -v -o servicelb-standalone .

FROM alpine:3.13
RUN apk add --no-cache ca-certificates findmnt
COPY --from=builder /usr/src/app/servicelb-standalone /bin/
ENTRYPOINT ["/bin/servicelb-standalone"]
