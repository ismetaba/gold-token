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
    -o /bin/mintburnd \
    ./services/mint-burn/cmd/mintburnd

# ─── Runtime stage ────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/mintburnd /usr/local/bin/mintburnd

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/mintburnd"]
