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
    -o /bin/admind \
    ./services/admin/cmd/admind

# ─── Runtime stage ────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/admind /usr/local/bin/admind

EXPOSE 8094

ENTRYPOINT ["/usr/local/bin/admind"]
