#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
env_file="$repo_root/.env"

if [ -e "$env_file" ]; then
    chmod 600 "$env_file"
    if ! awk -F= '$1 == "IPTV_ENCRYPTION_KEY" && length($2) > 0 { found=1 } END { exit !found }' "$env_file"; then
        printf '%s\n' 'Falta IPTV_ENCRYPTION_KEY. Recupera la clave original de una copia privada si ya hay fuentes guardadas.' >&2
        printf '%s\n' 'No se genera otra clave automáticamente: una clave distinta no puede descifrar las fuentes existentes.' >&2
        exit 1
    fi
    printf '%s\n' 'El archivo .env ya existe; se conserva sin cambios (permisos ajustados a 600).'
    printf '%s\n' 'Comprueba localmente BOOTSTRAP_ADMIN_USERNAME/PASSWORD e IPTV_ALLOWED_HOSTS antes de iniciar.'
    exit 0
fi

random_hex() {
    openssl rand -hex "$1"
}

local_db_password=$(random_hex 24)
session_secret=$(random_hex 48)
identity_key=$(random_hex 48)
iptv_encryption_key=$(openssl rand -base64 32)
bootstrap_password=$(random_hex 18)

umask 077
cat > "$env_file" <<EOF
LOCAL_POSTGRES_DB=walactv
LOCAL_POSTGRES_USER=walactv
LOCAL_POSTGRES_PASSWORD=$local_db_password
DATABASE_URL=postgres://walactv:$local_db_password@postgres:5432/walactv?sslmode=disable
APP_ORIGIN=http://127.0.0.1:8081
SESSION_SECRET=$session_secret
PROVIDER_IDENTITY_KEY=$identity_key
IPTV_ENCRYPTION_KEY=$iptv_encryption_key
IPTV_ALLOWED_HOSTS=
API_REPLICAS=1
BOOTSTRAP_ADMIN_USERNAME=admin
BOOTSTRAP_ADMIN_PASSWORD=$bootstrap_password
EOF
chmod 600 "$env_file"

printf '%s\n' 'Entorno local creado con permisos 600 en .env.'
printf '%s\n' 'Usuario inicial: admin. La contraseña se encuentra en .env; no se muestra aquí.'
printf '%s\n' 'IPTV_ALLOWED_HOSTS queda vacío: añade el host del proveedor manualmente en .env antes de conectar la lista.'
