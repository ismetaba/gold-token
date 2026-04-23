#!/bin/sh
# Creates the GOLD JetStream stream if it does not already exist.
# Runs once as an init container via natsio/nats-box.
set -e

SERVER="nats://nats:4222"

echo "Waiting for NATS at ${SERVER}..."
until nats --server="${SERVER}" account info > /dev/null 2>&1; do
  sleep 1
done
echo "NATS is ready."

# Create stream if it doesn't exist
if nats --server="${SERVER}" stream info GOLD > /dev/null 2>&1; then
  echo "Stream GOLD already exists — skipping."
else
  echo "Creating stream GOLD..."
  nats --server="${SERVER}" stream add GOLD \
    --subjects="gold.>" \
    --storage=file \
    --retention=limits \
    --discard=old \
    --replicas=1 \
    --max-age=168h \
    --dupe-window=2m \
    --max-msgs=-1 \
    --max-bytes=-1 \
    --max-msg-size=-1 \
    --defaults
  echo "Stream GOLD created."
fi
