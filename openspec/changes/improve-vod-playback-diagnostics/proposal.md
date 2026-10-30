## Why

Varias películas y series fallan pese a permitir CDN públicos dinámicos. La evidencia disponible incluye un `409 PLAYBACK_LIMIT`; todavía no demuestra una causa común en el proveedor. El proxy convierte causas diferentes en `502 UPSTREAM_ERROR` y el reproductor mezcla errores de transporte y compatibilidad. La política de medios admite solo puertos estándar y tres redirecciones. El usuario autoriza ampliar prudentemente esa compatibilidad y comprobar varios títulos.

## What Changes

- Conservar diagnósticos enumerados y saneados por reproducción autorizada, con distinción de estado upstream, red, política y navegador.
- Mostrar un mensaje útil para el límite de sesión, sin crear sesiones adicionales ni eludir límites del proveedor.
- Admitir una lista acotada de puertos públicos habituales de vídeo y hasta ocho redirecciones exclusivamente en medios autorizados.
- Validar películas y episodios, MP4 y MKV cuando existan, de forma secuencial, registrando reproducción efectiva además del estado HTTP.
- Determinar si quedan causas de contenedor/códec que necesitan otro cambio, sin atribuirlas a la política de red por anticipado.

## Capabilities

### New Capabilities
- `vod-playback-diagnostics`: diagnóstico privado y saneado de fallos, mensajes y matriz de comprobación VOD.

### Modified Capabilities
- `public-media-destinations`: puertos de vídeo públicos acotados y ocho redirecciones.

## Impact

API Go, reproductor Angular, pruebas y documentación local. Sin cambios de PostgreSQL, migraciones, servicios Compose o variables obligatorias. Se conserva `API_REPLICAS=1` y el migrador/bootstrap actual. No abarca transcodificación, remux, nuevos servicios, precarga masiva, publicación ni producción.
