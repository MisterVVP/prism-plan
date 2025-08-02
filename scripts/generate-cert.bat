@echo off
setlocal enabledelayedexpansion

REM === Names that must be accepted by the certificate ===
set "DNS1=localhost"
set "DNS2=mistervvp"
set "DNS3=mistervvp.local"

REM === Certificate lifetime (days) and output path ===
set "DAYS=365"
set "CERT_DIR=%~dp0..\certs"

if not exist "%CERT_DIR%" mkdir "%CERT_DIR%"

REM === Build a temporary OpenSSL config that includes both DNS SANs ===
set "CFG=%TEMP%\openssl_san.cnf"
> "%CFG%" (
    echo [req]
    echo default_bits = 2048
    echo prompt       = no
    echo default_md   = sha256
    echo distinguished_name = dn
    echo x509_extensions    = v3_req

    echo [dn]
    REM Use the first host as the common name (CN)
    echo CN = !DNS1!

    echo [v3_req]
    echo subjectAltName = @alt_names

    echo [alt_names]
    echo DNS.1 = !DNS1!
    echo DNS.2 = !DNS2!
    echo DNS.3 = !DNS3!
)

REM === Generate private key + certificate ===
openssl req -x509 -nodes -days %DAYS% ^
  -newkey rsa:2048 ^
  -keyout "%CERT_DIR%\nginx.key" ^
  -out    "%CERT_DIR%\nginx.crt" ^
  -config "%CFG%" -extensions v3_req

del "%CFG%"

REM === Trust the certificate (run this script from an elevated prompt) ===
echo Adding cert to Trusted Root Certification Authorities...
certutil -addstore -f "Root" "%CERT_DIR%\nginx.crt"

echo.
echo ✔ Certificate trusted for: %DNS1% and %DNS2% and %DNS3%
echo   Key : %CERT_DIR%\nginx.key
echo   Cert: %CERT_DIR%\nginx.crt
endlocal
