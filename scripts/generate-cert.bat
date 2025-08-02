@echo off
setlocal enabledelayedexpansion

:: ---------- Tweakables ----------
set "ROOT_CN=My-CA"
set "CA_DIR=%~dp0..\ca"
set "CERT_DIR=%~dp0..\certs"
set "CA_DAYS=3650"
set "SRV_DAYS=825"
:: ---------------------------------

if not exist "%CA_DIR%"  mkdir "%CA_DIR%"
if not exist "%CERT_DIR%" mkdir "%CERT_DIR%"

echo.
set /p SERVER_DNS=Enter the host-name or IP for the certificate (e.g. mistervvp.local): 
if "%SERVER_DNS%"=="" (
    echo [ERROR] Nothing entered. Aborting.
    exit /b 1
)

:: ---------- Root CA (create & trust if absent) ----------
if not exist "%CA_DIR%\rootCA.key" (
    echo [INFO] Generating root CA key ...
    openssl genrsa -out "%CA_DIR%\rootCA.key" 4096

    echo [INFO] Self-signing root certificate ...
    openssl req -x509 -new -nodes -key "%CA_DIR%\rootCA.key" ^
      -sha256 -days %CA_DAYS% -subj "/CN=%ROOT_CN%" ^
      -out "%CA_DIR%\rootCA.crt"

    openssl x509 -in "%CA_DIR%\rootCA.crt" -outform der -out "%CA_DIR%\rootCA.cer"

    echo [INFO] Adding root CA to Windows trust store ...
    certutil -addstore -f "Root" "%CA_DIR%\rootCA.crt"
) else (
    echo [OK] Root CA already exists — skipping CA creation.
)

:: ---------- Server cert for %SERVER_DNS% ----------
set "CFG=%TEMP%\openssl_srv.cnf"
> "%CFG%" (
    echo [req]
    echo default_bits = 2048
    echo prompt       = no
    echo default_md   = sha256
    echo distinguished_name = dn
    echo req_extensions      = v3
    echo.
    echo [dn]
    echo CN = %SERVER_DNS%
    echo.
    echo [v3]
    echo subjectAltName = @alt
    echo basicConstraints = CA:FALSE
    echo keyUsage = digitalSignature, keyEncipherment
    echo extendedKeyUsage = serverAuth
    echo.
    echo [alt]
    echo DNS.1 = %SERVER_DNS%
)

set "SRV_KEY=%CERT_DIR%\nginx.key"
set "SRV_CRT=%CERT_DIR%\nginx.crt"

echo [INFO] Generating server key + CSR ...
openssl req -new -nodes -keyout "%SRV_KEY%" ^
  -out "%CERT_DIR%\nginx.csr" ^
  -config "%CFG%"

echo [INFO] Signing server certificate ...
openssl x509 -req -in "%CERT_DIR%\nginx.csr" ^
  -CA "%CA_DIR%\rootCA.crt" -CAkey "%CA_DIR%\rootCA.key" ^
  -CAcreateserial -out "%SRV_CRT%" -days %SRV_DAYS% -sha256 ^
  -extfile "%CFG%" -extensions v3

del "%CFG%" "%CERT_DIR%\nginx.csr" "%CA_DIR%\rootCA.srl"

echo.
echo ============================================
echo   ✔  Finished
echo   Root CA  : %CA_DIR%\rootCA.crt
echo   Server   : %SRV_CRT%
echo   Key      : %SRV_KEY%
echo ============================================
echo ➊ Configure your web-server with the key & cert
echo ➋ Make %SERVER_DNS% resolve to this machine
echo ➌ Install rootCA.crt on other devices (iPhone, etc.)
echo.
endlocal
