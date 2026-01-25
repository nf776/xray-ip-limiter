FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS modules
WORKDIR /modules
COPY go.mod go.sum ./
RUN go mod download

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
WORKDIR /app
COPY --from=modules /go/pkg /go/pkg
COPY . .

RUN apk update && apk add --no-cache ca-certificates tzdata
RUN update-ca-certificates

ARG TARGETOS
ARG TARGETARCH
ARG VERSION

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-w -s -X main.Version=${VERSION:-dev} -X main.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
    -o /bin/app ./cmd/agent

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata ipset iptables

COPY --from=builder /bin/app /app/app

ENV DISABLE_ENV_FILE=true

CMD ["/app/app"]
