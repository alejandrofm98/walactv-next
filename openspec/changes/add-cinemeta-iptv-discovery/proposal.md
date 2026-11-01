# Proposal

## Why

Las categorías IPTV dependen del proveedor y dificultan descubrir películas y series. Cinemeta permite explorar obras por popularidad y género, siempre restringiendo los resultados a contenido disponible en la fuente privada del usuario.

## What Changes

- Añadir un modo explícito «Descubrir con Cinemeta» a Películas y Series, con popularidad, género, búsqueda y navegación incremental.
- Intersectar resultados públicos con IMDb ID estructurado del catálogo IPTV activo, por tipo y propietario; preservar las versiones IPTV y sus recursos de reproducción.
- Combinar género/búsqueda Cinemeta con categoría e idioma aproximado IPTV; comunicar cobertura parcial y ausencia de coincidencias verificadas.
- Mantener detalles localizados TMDB `es-ES` bajo demanda y reproducción IPTV independientes de Cinemeta.
- Excluir cambios en Directos, resolución difusa de títulos, indexación masiva externa, nuevos proveedores de streams, migraciones legacy, publicación y despliegue.

## Capabilities

### New Capabilities

- `iptv-cinemeta-discovery`: descubrimiento Cinemeta explícito y paginado, limitado a disponibilidad IPTV privada mediante identidad IMDb fiable.

### Modified Capabilities

Ninguna especificación principal publicada: `openspec/specs` no contiene capacidades. Este cambio complementa los deltas vigentes de `vod-catalog-navigation`, `catalog-navigation` y `on-demand-spanish-metadata`; no sustituye sus contratos.

## Impact

API Go: adaptador Cinemeta acotado y endpoint autenticado aditivo, reutilizando catálogo persistido y autorización. Angular standalone: controles y estado privado de descubrimiento, sin alterar las rutas de fichas y playback. No incorpora dependencias obligatorias, secretos, variables, servicios Compose ni cambios PostgreSQL: `imdb_id` y generación activa ya existen desde 004. El migrador existente permanece sin versiones nuevas. La PostgreSQL productiva continúa externa a Compose. Implementación y verificación local quedan separadas de cualquier push o despliegue.
