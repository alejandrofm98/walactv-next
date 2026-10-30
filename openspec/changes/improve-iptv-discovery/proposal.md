# Proposal

## Why

La fuente privada ya se guarda cifrada, pero solo persisten fragmentos de catálogo consultados y la búsqueda por categoría impide descubrir cómodamente el contenido. El usuario pide conservar la lista completa, reconocer canales por sus iconos y navegar películas y series con búsqueda, categorías e idiomas.

## What Changes

- Sincronizar catálogo IPTV completo live/movie/series a PostgreSQL, con estado visible, refresco asíncrono controlado y publicación atómica.
- Buscar en todo el tipo de catálogo y combinar categoría/idioma inferido antes de paginar.
- Mostrar logos/carátulas mediante imágenes autorizadas propias; originales privados cifrados y fallback visual.
- Completar secciones diferenciadas de películas y series, ficha basada en datos disponibles y acceso a reproducción/episodios.
- Investigar Cinemeta y el enfoque Nuvio como metadatos opcionales. La implementación actual conserva IPTV como catálogo reproducible; asociación externa futura por IMDb verificado.
- Migración aditiva 003, misma clave externa y mismos servicios Compose; sin publicar ni acceder a producción.

## Capabilities

### New Capabilities

- `persistent-iptv-discovery`: catálogo completo privado, sincronización y búsqueda/filtrado persistentes.
- `private-catalog-artwork`: imágenes de contenido privadas y seguras con fallback.
- `vod-catalog-navigation`: navegación y ficha de películas/series con procedencia honesta de metadatos.

### Modified Capabilities

Ninguna especificación consolidada existe aún en `openspec/specs`. Este cambio sustituye explícitamente la política de snapshots de navegación de 60 s descrita en el cambio previo `persist-private-iptv-sources`; conserva sus requisitos de cifrado, pertenencia, borrado y revalidación antes de reproducir.

## Impact

API Go (catálogo, persistencia, proxy de imágenes), migración PostgreSQL 003 y Angular (modelos, filtros, imágenes, fichas y estados). Contrato coordinado en `docs/catalog-contract.md`. No requiere Python, Redis, extensiones, nuevos secretos ni cambios de servicio Compose. No modifica credenciales guardadas ni esquema previo de biblioteca. Despliegue productivo y asociación automática Cinemeta quedan fuera del alcance.
