# Step 1: Build binary
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download

COPY . ./

ARG SERVICE_NAME
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./cmd/${SERVICE_NAME}

# Step 2: Runtime image
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /
COPY --from=builder /app/server /server

USER nonroot:nonroot

ENTRYPOINT ["/server"]
