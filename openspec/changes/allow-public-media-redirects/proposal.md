## Why

Las reproducciones IPTV fallan cuando el proveedor redirige a un CDN público ausente de `IPTV_ALLOWED_HOSTS`. Mantener manualmente todos sus servidores no escala y también impide HLS con recursos alojados en varios dominios.

## What Changes

- Autorizar destinos públicos dinámicos derivados exclusivamente de una reproducción autenticada: redirecciones HTTP y recursos declarados por manifiestos HLS.
- Separar el transporte de medios del transporte de control para conservar la allowlist del proveedor y sus restricciones de redirección.
- Conservar validación SSRF por destino/conexión, métodos GET/HEAD, puertos estándar, límites existentes, URLs opacas y aislamiento por usuario/playback.
- Incorporar pruebas deterministas y documentar el significado actualizado de `IPTV_ALLOWED_HOSTS`.

Fuera de alcance: proxy de URL arbitraria, puertos alternativos, bypass de conexiones del proveedor, transcodificación, cambios de catálogo/metadata/arte, publicación y despliegue productivo.

## Capabilities

### New Capabilities

- `public-media-destinations`: medios dinámicos públicos autorizados por procedencia, con protección SSRF independiente de la allowlist del control. Complementa `private-iptv-playback`, cuyo cambio inicial sigue sin archivar; precisa que la allowlist se aplica a la raíz/control y la procedencia autoriza derivados públicos.

## Impact

API Go: `playback.go`, transporte/validación compartida en `sources.go`, pruebas y documentación de configuración. Contrato web, PostgreSQL y fuentes cifradas compatibles. Sin variables nuevas, servicios Compose nuevos ni migraciones. `IPTV_ALLOWED_HOSTS` continúa siendo obligatorio para admitir la fuente/control; los CDN de medios no requieren inscripción manual. No modificar secretos reales ni imprimir URLs privadas. El stack local puede reconstruir/reiniciar API conservando la base y la clave existentes; sin push ni despliegue autorizado.
