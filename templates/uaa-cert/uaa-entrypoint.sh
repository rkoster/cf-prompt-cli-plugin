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

echo "=== Configuring Tomcat for SSL ==="
SERVER_XML="/layers/tanzu-buildpacks_apache-tomcat/catalina-base/conf/server.xml"

if [ -f "$SERVER_XML" ]; then
    echo "Modifying Tomcat server.xml to enable SSL..."
    
    # Backup original
    cp "$SERVER_XML" "$SERVER_XML.bak"
    
    # Replace HTTP connector with HTTPS connector
    sed -i 's|<Connector port='"'"'8080'"'"'|<Connector port="8443" protocol="org.apache.coyote.http11.Http11NioProtocol" SSLEnabled="true" scheme="https" secure="true" keystoreFile="'"$KEYSTORE_PATH"'" keystorePass="'"$KEYSTORE_PASSWORD"'" keystoreType="PKCS12" clientAuth="false" sslProtocol="TLS"|' "$SERVER_XML"
    
    echo "Tomcat SSL configuration complete!"
    echo "  Keystore: $KEYSTORE_PATH"
    echo "  Port: 8443"
else
    echo "WARNING: Tomcat server.xml not found at $SERVER_XML"
fi

exec "$@"
