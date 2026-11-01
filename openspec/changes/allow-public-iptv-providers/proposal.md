## Why

La instalación productiva rechaza proveedores válidos cuando falta `IPTV_ALLOWED_HOSTS`. El usuario quiere conectar sus proveedores públicos desde la web sin mantener una lista administrativa de dominios. La seguridad debe basarse también en validar el destino real de cada conexión.

## What Changes

- Admitir proveedores Xtream públicos cuando `IPTV_ALLOWED_HOSTS` falte o solo contenga entradas vacías; conservar una lista no vacía como restricción opcional retrocompatible.
- Aplicar la misma política al alta, restauración de fuentes cifradas, transporte de control y raíz de reproducción.
- Mantener formato `/get.php`, credenciales de proveedor, HTTP:80/HTTPS:443 para alta/control, origen fijo en sus redirecciones, resolución DNS pública y conexión a una IP validada.
- Probar el modo público con proveedores simulados, restricciones opcionales, SSRF y restauración, sin utilizar credenciales reales.
- Actualizar ejemplos y documentación para que la instalación no exija inscribir cada proveedor.

Fuera de alcance: M3U arbitrario sin API Xtream, puertos arbitrarios, redirecciones de control entre orígenes, cambios de cuentas o cifrado, retirada de defensas SSRF, backups, push y despliegue productivo.

## Capabilities

### New Capabilities

- `public-iptv-providers`: proveedores públicos sin lista obligatoria de dominios, con restricción opcional y controles de red por conexión. Precisa los cambios abiertos `private-iptv-playback`, `saved-private-sources` y `public-media-destinations`: la lista deja de ser requisito para alta/control y raíz; el control mantiene origen fijo y los medios mantienen autorización por procedencia.

## Impact

API Go y pruebas de política de fuentes/control, restauración y raíz de playback; `.env.example` y documentación de configuración/despliegue. No hay cambios de contrato web, esquema PostgreSQL, migraciones, nuevos servicios Compose o variables obligatorias. La configuración existente con lista no vacía conserva su restricción; al retirar la variable se activa el modo público. La publicación requiere autorización y destino verificado por el coordinador.
