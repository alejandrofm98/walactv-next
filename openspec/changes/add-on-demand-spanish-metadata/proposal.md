# Proposal

## Why

Las fichas actuales conservan datos IPTV y reproducción, pero carecen de sinopsis y otros detalles en español. La investigación `docs/cinemeta-research.md` recomienda TMDB oficial desde Go, consultado al abrir contenido y con caché persistente; Cinemeta público no garantiza español. El usuario autorizó implementar este enriquecimiento sin recorrer el catálogo global.

## What Changes

- Enriquecer fichas de películas/series en `es-ES` y temporadas abiertas con TMDB oficial desde backend, conservando IPTV como disponibilidad y reproducción.
- Abrir la ficha muestra inmediatamente los datos IPTV; una petición independiente obtiene metadata y actualiza la ficha sin bloquear reproducción. Listados, importación y arranque no consultan TMDB.
- Resolver identidades con IDs externos verificables o coincidencia única de título exacto normalizado y año conocido. Ambigüedad o evidencia insuficiente mantiene ficha IPTV, sin asociación automática.
- Persistir fichas canónicas localizadas, temporadas y decisiones privadas de resolución en PostgreSQL; caché negativa, stale acotado, deduplicación y límites ante fallos/429.
- Añadir migración aditiva 004 y secreto opcional `TMDB_API_TOKEN` únicamente en API; sin token continúa funcionando IPTV.
- Mostrar procedencia y atribución TMDB. Actualizar después de implementar el contrato y documentación operativa afectados.

## Capabilities

### New Capabilities

- `on-demand-spanish-metadata`: resolución conservadora, fichas españolas bajo demanda, caché persistente y degradación segura.

### Modified Capabilities

No existen specs consolidadas. Este cambio complementa `vod-catalog-navigation` de `improve-iptv-discovery`: abrir una ficha constituye la acción explícita para consultar el título mínimo a TMDB; mantiene las prohibiciones de envío masivo, asociación dudosa y dependencia de metadatos para reproducir. Sustituye únicamente la investigación pendiente de integración; no altera disponibilidad IPTV.

## Impact

API Go, PostgreSQL, Angular, documentación de contrato/despliegue, `.env.example` y entorno del servicio API en `compose.yml`. PostgreSQL externa productiva y override local permanecen independientes; `API_REPLICAS=1` continúa obligatorio. Migración 004 antes de API con migrador existente y backup local privado; imagen anterior ignora tablas añadidas y rollback no revierte esquema.

Fuera de alcance: crawl global TMDB/IPTV, precarga automática, traducción propia, cambio de idioma por perfil, selección manual de candidatos, temporadas completas anticipadas, trailers/watch providers, cambios de streams o credenciales IPTV, otro servicio/Redis, integración de addons públicos, push y despliegue. Producción y licencia para eventual uso comercial requieren revisión antes de publicar.
