FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X gpm.sh/gpm/gpm-cli/cmd.Version=${VERSION} -X gpm.sh/gpm/gpm-cli/cmd.Commit=${COMMIT} -X gpm.sh/gpm/gpm-cli/cmd.Date=${DATE} -s -w" \
    -o gpm .

FROM alpine:3.18

RUN apk --no-cache add ca-certificates

RUN addgroup -g 1001 -S gpm && \
    adduser -u 1001 -S gpm -G gpm

WORKDIR /home/gpm

COPY --from=builder /app/gpm /usr/local/bin/gpm

RUN chown gpm:gpm /usr/local/bin/gpm

USER gpm

ENTRYPOINT ["gpm"]
CMD ["--help"]

LABEL maintainer="GPM Team"
LABEL description="GPM CLI - Game Package Manager"
LABEL version="${VERSION}"
