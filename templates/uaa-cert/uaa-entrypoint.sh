#!/bin/bash
set -euo pipefail

KEYSTORE_PATH="${KEYSTORE_PATH:-/workspace/keystore.p12}"
KEYSTORE_PASSWORD="${KEYSTORE_PASSWORD:-changeit}"

echo "=== UAA SSL Entrypoint Wrapper ==="
echo "Generating keystore at: $KEYSTORE_PATH"

if [ ! -f "$KEYSTORE_PATH" ]; then
    if [ -f "/etc/ssl-certs/uaa.crt" ] && [ -f "/etc/ssl-certs/uaa.key" ]; then
        echo "Converting PEM certificate and key to PKCS12 keystore..."
        
        openssl pkcs12 -export \
            -in /etc/ssl-certs/uaa.crt \
            -inkey /etc/ssl-certs/uaa.key \
            -out "$KEYSTORE_PATH" \
            -name uaa \
            -password "pass:$KEYSTORE_PASSWORD"
        
        chmod 644 "$KEYSTORE_PATH"
        echo "Keystore generated successfully!"
    else
        echo "ERROR: Certificate files not found at /etc/ssl-certs/uaa.crt and /etc/ssl-certs/uaa.key"
        exit 1
    fi
else
    echo "Keystore already exists at $KEYSTORE_PATH, skipping generation"
fi

echo "=== Starting UAA with SSL configuration ==="
echo "  SERVER_PORT: ${SERVER_PORT:-8443}"
echo "  SERVER_SSL_ENABLED: ${SERVER_SSL_ENABLED:-true}"
echo "  SERVER_SSL_KEY_STORE: ${SERVER_SSL_KEY_STORE:-$KEYSTORE_PATH}"
echo "  SERVER_SSL_KEY_STORE_TYPE: ${SERVER_SSL_KEY_STORE_TYPE:-PKCS12}"

exec "$@"
