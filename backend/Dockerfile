# Build a static binary (CGO disabled) so the runtime image needs no libc.
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o cinematch-backend .

# distroless/static has no shell or package manager.
FROM gcr.io/distroless/static-debian12

WORKDIR /app
COPY --from=builder /app/cinematch-backend .

EXPOSE 8080

ENTRYPOINT ["/app/cinematch-backend"]
