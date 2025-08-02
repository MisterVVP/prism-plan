#!/usr/bin/env bash
set -euo pipefail

# === Names that must be accepted by the certificate ===
DNS1="localhost"
DNS2="mistervvp"
DNS3="mistervvp.local"

DAYS="365"
CERT_DIR="$(dirname "$0")/../certs"
mkdir -p "$CERT_DIR"

# --- Build a temporary OpenSSL config with both DNS SANs ---
TMP_CFG="$(mktemp)"
cat >"$TMP_CFG" <<EOF
[req]
default_bits = 2048
prompt       = no
default_md   = sha256
distinguished_name = dn
x509_extensions    = v3_req

[dn]
CN = $DNS1

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = $DNS1
DNS.2 = $DNS2
DNS.3 = $DNS3
EOF

# --- Generate key + cert ---
openssl req -x509 -nodes -days "$DAYS" \
  -newkey rsa:2048 \
  -keyout "$CERT_DIR/nginx.key" \
  -out    "$CERT_DIR/nginx.crt" \
  -config "$TMP_CFG" -extensions v3_req
rm "$TMP_CFG"

# --- Trust it (root password required) ---
echo "Installing certificate to system trust store..."
sudo cp "$CERT_DIR/nginx.crt" /usr/local/share/ca-certificates/nginx.crt
sudo update-ca-certificates         # Fedora/RHEL: sudo update-ca-trust

echo "✔ Certificate trusted for: $DNS1 and $DNS2 and $DNS3"
echo "  Key : $CERT_DIR/nginx.key"
echo "  Cert: $CERT_DIR/nginx.crt"
