FROM golang:1.24-alpine AS builder
WORKDIR /app

# Copy go mod files first and download dependencies
# This creates a separate layer that only invalidates when dependencies change
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

ARG GO_BUILD_TAGS
RUN go build ${GO_BUILD_TAGS:+-tags="$GO_BUILD_TAGS"} -o /build/registry ./cmd/registry

FROM alpine:latest
WORKDIR /app
COPY --from=builder /build/registry .
COPY --from=builder /app/data/seed.json /app/data/seed.json

# Create a non-privileged user that the app will run under.
# See https://docs.docker.com/go/dockerfile-user-best-practices/
ARG UID=10001
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    appuser

USER appuser
EXPOSE 8080

ENTRYPOINT ["./registry"]
