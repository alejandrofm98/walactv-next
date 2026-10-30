## Why

La comprobación local identifica un fallo de compatibilidad real: una película MKV 4K con HEVC y AC3/EAC3 recibe bytes mediante 206 pero Chrome informa MediaError 4; una versión 1080 reproduce. Ampliar puertos no resuelve ese archivo. Se necesita una conversión opcional del VOD al formato que admita el navegador, acotando los recursos y respetando una conexión por cuenta.

## What Changes

- Fallback automático, una vez por intento, desde un fallo observado de formato/decodificación de VOD a HLS con vídeo H.264 y audio AAC.
- FFmpeg instalado en imagen API y gestionado por Go, sin URL o credenciales del proveedor en argumentos, entorno de hijo, archivos o logs.
- Gateway interno por trabajo que conserva la política pública de medios y serializa conexiones upstream.
- Máximo un trabajo global, perfil hasta 720p, dos hilos de codificación, presupuesto de tiempo/memoria/almacenamiento y limpieza al cerrar.
- Avance temporal mediante reinicio del trabajo en offset, con generaciones aisladas y posición lógica; no emular Range aleatorio sobre bytes transcodificados.

## Capabilities

### New Capabilities
- `vod-transcoding`: conversión VOD privada y acotada, fallback, seek, recursos y operación local.

### Modified Capabilities
- Ninguna capacidad canónica adicional; se integra con las reproducciones y su diagnóstico existentes.

## Impact

Go, Angular, imagen API y configuración Compose. Sin migraciones PostgreSQL ni persistencia de medios; producción externa intacta. Nueva configuración no secreta `VOD_TRANSCODING_ENABLED` con valor por defecto false y activación local explícita. No incluye directo, DRM, GPU, varios trabajos, elección de pistas, subtítulos quemados ni garantía de tiempo real para todo archivo 4K. Sin publicación o despliegue productivo.
