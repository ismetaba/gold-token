# ─── Build stage ─────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Cache module downloads before copying source
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /bin/orderd \
    ./services/order/cmd/orderd

# ─── Runtime stage ────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/orderd /usr/local/bin/orderd

EXPOSE 8085

ENTRYPOINT ["/usr/local/bin/orderd"]
