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
    -o /bin/walletd \
    ./services/wallet/cmd/walletd

# ─── Runtime stage ────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/walletd /usr/local/bin/walletd

EXPOSE 8083

ENTRYPOINT ["/usr/local/bin/walletd"]
