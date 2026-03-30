# Stage 1: build the binary.
# Using golang:1.22-alpine keeps the builder image small while providing CGO
# and standard tooling. CGO_ENABLED=0 produces a fully static binary.
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o cinematch-backend .

# Stage 2: minimal runtime image.
# distroless/static contains no shell, package manager, or OS tools,
# which dramatically shrinks the attack surface for a production service.
FROM gcr.io/distroless/static-debian12

WORKDIR /app
COPY --from=builder /app/cinematch-backend .

EXPOSE 8080

ENTRYPOINT ["/app/cinematch-backend"]
