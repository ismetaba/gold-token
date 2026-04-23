# SoftHSM2 local dev container
# Provides a PKCS#11-compatible software HSM for key management in local dev.
# Mount /var/lib/softhsm/tokens as a named volume to persist tokens across restarts.
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
        softhsm2 \
        openssl \
    && rm -rf /var/lib/apt/lists/*

# Create directories and default softhsm2.conf
RUN mkdir -p /var/lib/softhsm/tokens && \
    mkdir -p /etc/softhsm2 && \
    echo "directories.tokendir = /var/lib/softhsm/tokens/" > /etc/softhsm2/softhsm2.conf && \
    echo "objectstore.backend = file" >> /etc/softhsm2/softhsm2.conf && \
    echo "log.level = INFO" >> /etc/softhsm2/softhsm2.conf

ENV SOFTHSM2_CONF=/etc/softhsm2/softhsm2.conf

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

VOLUME ["/var/lib/softhsm/tokens"]

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
