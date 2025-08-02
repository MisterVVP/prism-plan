#!/usr/bin/env bash
# ==========================================================
#  make_ca_and_server.sh   (run once; needs sudo to trust CA)
# ==========================================================
set -euo pipefail

# -------- One-time settings you may tweak ----------
ROOT_CN="Prism-Plan-CA"
CA_DIR="$(dirname "$0")/../ca"
CERT_DIR="$(dirname "$0")/../certs"
CA_DAYS=3650
SRV_DAYS=825
# ---------------------------------------------------

mkdir -p "$CA_DIR" "$CERT_DIR"

# ---------------------------------------------------
# Ask user for the server DNS / IP
# ---------------------------------------------------
echo
read -rp "Enter the host-name or IP for the certificate (e.g. mistervvp.local): " SERVER_DNS
if [[ -z "$SERVER_DNS" ]]; then
  echo "[ERROR] Nothing entered. Aborting." >&2
  exit 1
fi

# ---------------------------------------------------
# 1) Root CA (create & trust if absent)
# ---------------------------------------------------
if [[ ! -f "$CA_DIR/rootCA.key" ]]; then
  echo "[INFO] Generating root CA key ..."
  openssl genrsa -out "$CA_DIR/rootCA.key" 4096

  echo "[INFO] Self-signing root certificate ..."
  openssl req -x509 -new -nodes -key "$CA_DIR/rootCA.key" \
    -sha256 -days "$CA_DAYS" -subj "/CN=$ROOT_CN" \
    -out "$CA_DIR/rootCA.crt"

  openssl x509 -in "$CA_DIR/rootCA.crt" -outform der -out "$CA_DIR/rootCA.cer"

  echo "[INFO] Trusting root CA system-wide (sudo) ..."
  sudo cp "$CA_DIR/rootCA.crt" /usr/local/share/ca-certificates/
  sudo update-ca-certificates    # Fedora/RHEL: sudo update-ca-trust
else
  echo "[OK] Root CA already exists — skipping CA creation."
fi

# ---------------------------------------------------
# 2) Server certificate for $SERVER_DNS
# ---------------------------------------------------
CFG="$(mktemp)"
cat >"$CFG" <<EOF
[req]
default_bits = 2048
prompt       = no
default_md   = sha256
distinguished_name = dn
req_extensions = v3

[dn]
CN = $SERVER_DNS

[v3]
subjectAltName = @alt
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth

[alt]
DNS.1 = $SERVER_DNS
EOF

SRV_KEY="$CERT_DIR/nginx.key"
SRV_CRT="$CERT_DIR/nginx.crt"

echo "[INFO] Generating server key + CSR ..."
openssl req -new -nodes -keyout "$SRV_KEY" \
  -out "$CERT_DIR/nginx.csr" -config "$CFG"

echo "[INFO] Signing server certificate ..."
openssl x509 -req -in "$CERT_DIR/nginx.csr" \
  -CA "$CA_DIR/rootCA.crt" -CAkey "$CA_DIR/rootCA.key" \
  -CAcreateserial -out "$SRV_CRT" -days "$SRV_DAYS" -sha256 \
  -extfile "$CFG" -extensions v3

rm "$CFG" "$CERT_DIR/nginx.csr" "$CA_DIR/rootCA.srl"

cat <<EOF

============================================
 ✔  Finished
 Root CA  : $CA_DIR/rootCA.crt
 Server   : $SRV_CRT
 Key      : $SRV_KEY
============================================
1) Point your web-server at the key & cert above.
2) Make $SERVER_DNS resolve to this machine
   (mDNS/Bonjour, router DNS, /etc/hosts, etc.).
3) Install rootCA.crt on other devices (iPhone, etc.),
   then visit  https://$SERVER_DNS:8080
============================================
EOF
