# Validation may pin these defaults to the digests pulled by the hosted job.
ARG GO_BUILDER_IMAGE=golang:1.27.3-alpine
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian12:nonroot

# Step 1: Build binary
FROM ${GO_BUILDER_IMAGE} AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download

COPY . ./

ARG SERVICE_NAME
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./cmd/${SERVICE_NAME}

# Step 2: Runtime image
FROM ${RUNTIME_IMAGE}

WORKDIR /
COPY --from=builder /app/server /server

USER nonroot:nonroot

ENTRYPOINT ["/server"]
