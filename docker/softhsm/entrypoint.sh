#!/bin/sh
# Initializes the SoftHSM2 token on first start, then sleeps so the container
# stays up (other services can copy the PKCS#11 library path from here).
set -e

TOKEN_LABEL="gold-dev"
TOKEN_PIN="1234"
TOKEN_SO_PIN="12345678"

if ! softhsm2-util --show-slots | grep -q "Label:.*${TOKEN_LABEL}"; then
  echo "Initializing SoftHSM2 token '${TOKEN_LABEL}'..."
  softhsm2-util --init-token --free \
    --label "${TOKEN_LABEL}" \
    --pin "${TOKEN_PIN}" \
    --so-pin "${TOKEN_SO_PIN}"
  echo "Token initialized."
else
  echo "SoftHSM2 token '${TOKEN_LABEL}' already exists."
fi

echo "PKCS#11 library: $(find /usr -name 'libsofthsm2.so' 2>/dev/null | head -1)"
echo "SoftHSM2 ready."

# Keep container running
exec sleep infinity
