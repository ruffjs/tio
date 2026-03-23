#!/bin/bash

# Generate CA, server, and client certificates for mTLS testing
# This script creates all necessary certificates for MQTT mutual TLS authentication

set -e

CERT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$CERT_DIR"

CLIENT_CN="${CLIENT_CN:-mtls-client}"
SERVER_CN="${SERVER_CN:-localhost}"

echo "=== Generating Certificates for mTLS Demo ==="
echo "Client certificate CN: ${CLIENT_CN}"
echo "Server certificate CN: ${SERVER_CN}"

# 1. Generate CA private key and certificate
echo "[1/6] Generating CA private key and certificate..."
openssl genrsa -out ca-key.pem 4096
openssl req -new -x509 -key ca-key.pem -out ca.pem -days 3650 \
    -subj "/C=CN/ST=Beijing/L=Beijing/O=mTLS Demo/OU=Test/CN=mTLS Demo CA"

# 2. Generate server private key and CSR
echo "[2/6] Generating server private key and CSR..."
openssl genrsa -out server-key.pem 4096
openssl req -new -key server-key.pem -out server-req.pem \
    -subj "/C=CN/ST=Beijing/L=Beijing/O=mTLS Demo/OU=Server/CN=${SERVER_CN}"

# 3. Sign server certificate with CA
echo "[3/6] Signing server certificate with CA..."
openssl x509 -req -in server-req.pem -CA ca.pem -CAkey ca-key.pem \
    -CAcreateserial -out server-cert.pem -days 3650 \
    -extfile <(echo "subjectAltName=DNS:${SERVER_CN},DNS:localhost,IP:127.0.0.1")

# 4. Generate client private key and CSR
echo "[4/6] Generating client private key and CSR..."
openssl genrsa -out client-key.pem 4096
openssl req -new -key client-key.pem -out client-req.pem \
    -subj "/C=CN/ST=Beijing/L=Beijing/O=mTLS Demo/OU=Client/CN=${CLIENT_CN}"

# 5. Sign client certificate with CA
echo "[5/6] Signing client certificate with CA..."
openssl x509 -req -in client-req.pem -CA ca.pem -CAkey ca-key.pem \
    -CAcreateserial -out client-cert.pem -days 3650

# 6. Clean up CSR files
echo "[6/6] Cleaning up..."
rm -f server-req.pem client-req.pem

# Set appropriate permissions
chmod 600 *-key.pem
chmod 644 *.pem

echo ""
echo "=== Certificates Generated Successfully ==="
echo ""
echo "Generated files:"
ls -la *.pem
echo ""
echo "Certificate usage:"
echo "  - CA Certificate:        ca.pem"
echo "  - Server Certificate:    server-cert.pem"
echo "  - Server Key:            server-key.pem"
echo "  - Client Certificate:    client-cert.pem"
echo "  - Client Key:            client-key.pem"
echo "  - Device thingId (CN):   ${CLIENT_CN}"
echo ""
echo "To verify certificates:"
echo "  openssl verify -CAfile ca.pem server-cert.pem"
echo "  openssl verify -CAfile ca.pem client-cert.pem"
