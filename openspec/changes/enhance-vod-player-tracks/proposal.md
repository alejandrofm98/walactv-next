# Proposal

## Why

El reproductor VOD conserva reproducción y progreso, pero sus controles nativos y los de conversión forman experiencias distintas y no ofrecen selección explícita de pistas disponibles. El usuario quiere una composición más inmersiva y poder elegir audio y subtítulos cuando el medio y el motor realmente lo permitan.

## What Changes

- Unificar la composición de películas y episodios: vídeo dominante, controles discretos, título y una salida clara, con identidad cálida propia de WalacTV y affordances generales de reproducción como referencia.
- Ofrecer audio seleccionable cuando haya al menos dos pistas controlables y subtítulos cuando haya al menos una pista controlable, con opción de desactivarlos. Identificar idioma solo a partir de datos reales.
- Detectar pistas de HLS mediante el motor vigente y de reproducción nativa solo mediante capacidades verificables del navegador; mostrar límites claros sin selectores ficticios.
- Incorporar inspección acotada de pistas MP4/MKV autorizadas mediante ffprobe y gateway privado; ofrecer elección de audio y subtítulos textuales soportados mediante la conversión VOD existente, reiniciando en la posición lógica actual.
- Preservar progreso lógico, reanudación, límites, errores y fullscreen; explicar pistas de imagen, codecs no soportados, límites de inspección y conversión deshabilitada.
- Excluir Live, fuentes externas de subtítulos, nuevas preferencias persistentes, cambios de catálogo y publicación.

## Capabilities

### New Capabilities

- `vod-player-experience`: composición inmersiva VOD, controles accesibles y selección honesta de pistas según motor y medio.

### Modified Capabilities

Ninguna: `openspec list --specs` no tiene capacidades principales. Este cambio complementa los artefactos existentes `immersive-web-experience`, `vod-transcoding` y `private-iptv-playback`; habilita selectores únicamente con la capacidad real que esos planes exigían como condición.

## Impact

Frontend del reproductor y API Go VOD: inventario efímero de pistas, opciones aditivas de conversión y entrega privada de subtítulos textuales. Se conserva hls.js 1.7.3; la imagen API verifica ffprobe y codificador WebVTT del paquete FFmpeg existente. Se reutilizan gateway, slot global, presupuestos y `VOD_TRANSCODING_ENABLED`; no hay servicios, puertos, Compose, variables, PostgreSQL ni migraciones nuevos. No cambia Live y no se autoriza push o despliegue.
