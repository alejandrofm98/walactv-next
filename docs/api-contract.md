# Contrato API local v1

Contrato de implementación del 2026-10-28. Base `/api/v1`, JSON UTF-8 sin envelope de éxito. Errores sanitizados: `{ "error": { "code": "CODIGO", "message": "Mensaje seguro" } }`; nunca URL privada, credenciales ni error bruto proveedor. IDs strings opacos; fechas ISO 8601 UTC. Cookies del mismo origen; ninguna respuesta entrega secretos del proveedor.

## Sesión y protección

Cookie `walactv_session`, HttpOnly, SameSite=Strict, Path=/; Secure en HTTPS/producción, HTTP solo local. Login público por POST; no registro público. Todas las mutaciones autenticadas exigen `X-CSRF-Token` y validación de `Origin` exacto con `APP_ORIGIN`; login también valida origen. Las lecturas/media autorizan cookie y pertenencia. La UI usa credenciales same-origin. TTL app: 30 min inactivo, 12 h absoluto; heartbeat válido renueva inactividad, no el absoluto. La copia RAM de fuente tiene TTL inactivo 30 min; se renueva mediante actividad/heartbeat sin superar autenticación padre y puede hidratarse desde PostgreSQL cifrado tras expirar. La fuente guardada no caduca por logout/reinicio.

| Método y ruta | Entrada | Éxito |
|---|---|---|
| POST `/auth/login` | `{username,password}` | 200 `{user:{id,username,role},csrfToken}` + cookie |
| GET `/auth/me` | Cookie | 200 mismo objeto de sesión |
| POST `/auth/logout` | Cookie+CSRF | 204, revoca medios y copia RAM; conserva fuente guardada y elimina cookie |
| GET `/sources` | Cookie | 200 `{sources:[Source]}` |
| POST `/sources` | `{playlistUrl}` | 201 `Source` |
| DELETE `/sources/{id}` | Cookie+CSRF | 204, libera reproducción propia y borra fuente/catálogo persistentes |
| GET `/sources/{id}/categories?type=live\|movie\|series` | Cookie | 200 `{categories:[{id,name}]}` |
| GET `/sources/{id}/items?type=...&categoryId=...&page=1&pageSize=40&search=...` | Cookie | 200 `{items:[Item],total,page,pageSize}` |
| GET `/sources/{id}/series/{seriesId}/episodes` | Cookie | 200 `{episodes:[Episode]}` |
| GET `/sources/{id}/items/{kind}/{itemId}/metadata` | Cookie, kind `movie\|series` | 200 `MetadataResponse`; consulta bajo demanda al abrir una ficha |
| GET `/sources/{id}/items/series/{itemId}/seasons/{season}/metadata` | Cookie; temporada 0..99 presente en IPTV | 200 `MetadataResponse` con la temporada consultada |
| POST `/playbacks` | `{sourceId,kind,itemId,seriesId?}` | 201 `Playback` |
| POST `/playbacks/{id}/heartbeat` | Cookie+CSRF | 204 |
| DELETE `/playbacks/{id}` | Cookie+CSRF | 204, cierre upstream confirmado |
| POST `/playbacks/{id}/tracks` | Cookie+CSRF, `{}` | 200 inventario efímero saneado; solo archivo propio con conversión habilitada |
| POST `/playbacks/{id}/transcode` | Cookie+CSRF, `{offsetSeconds,trackRevision?,audioTrackId?,subtitleTrackId?}` | 200 vista de conversión; campos de pista opcionales y validados contra el inventario vigente |
| GET `/playbacks/{id}/transcode/{generation}/subtitles.vtt?at=...` | Cookie | Ventana WebVTT temporal y saneada de la generación propia; `no-store` |
| GET/HEAD `/media/{playback}/{asset}` | Cookie | HLS/archivo autorizado, HEAD/Range/206/416 |
| GET `/favorites` | Cookie | 200 `{favorites:[Favorite]}` |
| POST `/favorites` | `{contentKey,title,kind}` | 201 `Favorite`, idempotente por usuario/contentKey |
| DELETE `/favorites/{id}` | Cookie+CSRF | 204 |
| GET `/progress` | Cookie | 200 `{progress:[Progress]}` |
| PUT `/progress` | `{contentKey,title,kind,positionSeconds,durationSeconds}` | 200 `Progress`, upsert |
| GET `/acestream/channels` | Cookie | 200 `{channels:[{id,name,contentId}]}` de catálogo de ejemplo público sin fuente privada |
| GET `/health` | Pública | 200 `{status:"ok"}` o 503 si no saludable |

Además `/healthz` público puede ser alias operativo para Compose. Métodos desconocidos 405, auth ausente/expirada 401, CSRF/origen 403, recurso ajeno 404, entrada inválida 400, lease ocupado 409, catálogo excesivo 413, upstream 502/timeout 504.

## Modelos

- `Source = {id,label,accountKey,expiresAt,saved:true}`. `accountKey` es identidad HMAC estable sobre origen+cuenta canónica sin contraseña, con `PROVIDER_IDENTITY_KEY` persistente. ID fuente es persistente y propio del usuario; expiresAt describe solo la caché RAM, no caducidad de la fuente guardada; UI nunca necesita datos privados proveedor.
- `Item = {id,title,kind,containerExtension?,contentKey?}`. `kind` catálogo: `live|movie|series`; IDs únicamente del catálogo servidor. `Episode = {id,title,season,episode,containerExtension,contentKey?}` con números season/episode.
- `Playback = {id,url,format,title,contentKey,expiresAt}`. `url` relativa opaca bajo `/api/v1/media/`; `format=hls|file`. Entrada kind playback: `live|movie|episode`; para episodio se aporta seriesId. Extensión/URL proveedor proceden de catálogo validado/cacheado en servidor, no del cliente.
- El inventario de pistas de archivo es efímero y explícito: devuelve IDs opacos, tipo y etiquetas saneadas, nunca URL, ruta ni tags arbitrarios. La revisión e IDs solo valen para ese playback. La inspección se desactiva junto con `VOD_TRANSCODING_ENABLED`; HLS y formatos nativos exponen las capacidades propias del motor.
- `POST /playbacks/{id}/transcode` mantiene `{offsetSeconds}` para clientes anteriores. Los IDs opcionales de pista se resuelven en servidor contra la revisión vigente; el cliente no puede enviar mapas ni índices FFmpeg. Solo audio y subtítulos textuales reconocidos son seleccionables.
- `Favorite = {id,contentKey,title,kind}`. `Progress = {contentKey,title,kind,positionSeconds,durationSeconds,updatedAt}`. kind biblioteca: `live|movie|episode`. Posición/duración son segundos finitos no negativos; no mezclar unidades. `contentKey` estable namespaced por cuenta+tipo+ID y usuario en almacenamiento; backend valida que pertenece al catálogo accesible. No usar ID fuente temporal como identidad persistente.

## Reproducción

Una fuente activa por usuario. Fuentes cifradas y catálogo saneado se guardan en PostgreSQL; estado descifrado activo y leases RAM: un proceso/réplica API, mutex seguro, sin restricción GOMAXPROCS. Lease por cuenta global entre usuarios/pestañas; límite proveedor observado uno. La UI espera DELETE antes de POST nuevo. Heartbeat 10 s, inactividad playback 30 s; finalizar/revocar/logout cancela upstream. Un fragmento abortado por seek/ABR cierra solo esa request, no el lease. Sesión/cuenta no válida impide medios aunque quede cookie.

HLS reescribe variantes, claves, mapas y segmentos a assets opacos; archivos conservan Range/HEAD. No se exige FFmpeg en este núcleo; formatos incompatibles dan error honesto. Sin timeout total corto para vídeos; control catálogo 15 s/5 MB. Allowlist y DNS/IP/redirect validados en Go, sin proxy URL libre ni forward de cookie al proveedor. Fixtures locales requieren configuración de prueba explícita que jamás se activa en producción.

## Operación local

CLI Go propuesta `walactv migrate` y `walactv bootstrap-admin`. Migraciones SQL versionadas con bloqueo único antes de `walactv serve`; no desde cada réplica. Bootstrap consume `BOOTSTRAP_ADMIN_USERNAME`/`BOOTSTRAP_ADMIN_PASSWORD` del entorno local ignorado y no los imprime. PostgreSQL local nuevo en override, nunca base anterior. Puertos: web `127.0.0.1:8081`, API solo red interna por Nginx. Producción necesitará PostgreSQL externo y destino Dokploy todavía no definido.

## Biblioteca y reproducción desde el catálogo

`contentKey` opcional en Item/Episode permite comparar favoritos exactamente sin deducir IDs de su HMAC ni abrir un playback solo para guardar un favorito. La biblioteca lista favoritos/progreso persistentes; volver a seleccionar el mismo contenido del catálogo recupera el progreso por `Playback.contentKey`. La reproducción directa desde una referencia opaca de biblioteca requerirá una ampliación futura del contrato, no se presenta como acción disponible.

## Metadata española bajo demanda

Los endpoints de metadata son lecturas autenticadas. El servidor valida primero que la fuente pertenece a la sesión y que tipo/ID están en la generación activa; solo entonces consulta decisión privada o caché canónica. Un elemento ajeno o inexistente responde 404. `MetadataResponse={status:"ready|stale|unresolved|ambiguous|disabled|unavailable",language?:"es-ES",source?:"TMDB",metadata?:Metadata,season?:Season}`. Los estados sin detalle no cambian el catálogo ni la reproducción. Las fichas frías esperan una consulta acotada del API; una ficha cacheada stale responde inmediatamente y refresca en segundo plano.

`Metadata={id,kind,title?,originalTitle?,overview?,releaseDate?,year?,genres?,runtime?,rating?,ratingSource?,posterUrl?,backdropUrl?,credits?}`. Se omiten campos localizados vacíos; la sinopsis vacía no se rellena con inglés. Arte acepta solo rutas relativas normalizadas del host HTTPS de TMDB. `Season={number,name?,overview?,episodes:[{number,name?,overview?,stillUrl?,airDate?}]}`; la web lo solicita solo tras seleccionar una temporada existente de IPTV y solo muestra nombres para números de episodio que ya existen en IPTV. TMDB nunca crea disponibilidad ni recursos de reproducción.

El API no consulta títulos durante importación, listados, búsqueda, inicio o reproducción. La búsqueda automática requiere título exacto normalizado y año coincidente; sin año solo se acepta un ID externo estructurado presente en el proveedor y verificado contra el detalle. ID IPTV no es ID TMDB. La caché pública dura 7 días y puede mostrarse stale hasta 30 días; decisiones `unresolved`/`ambiguous` duran 24 horas. Fallos externos no sustituyen datos previos. Las respuestas de metadata no llevan cache público. La interfaz informa de la consulta externa y atribuye TMDB sin afirmar respaldo o certificación.

## Persistencia privada de fuentes

POST /sources conserva por defecto la URL original/lista, usuario y contraseña **cifrados en Go** con AES-256-GCM, nonce aleatorio y AAD versión+userID+sourceID; no hay checkbox adicional. `IPTV_ENCRYPTION_KEY` es base64 de 32 bytes, estable y externo a PostgreSQL/Git. Ningún endpoint devuelve datos descifrados. GET /sources recupera únicamente fuentes del propietario tras login/reinicio sin exigir proveedor accesible. Fallo de clave/ciphertext deniega el acceso sin borrar o sobrescribir datos.

Catálogo PostgreSQL JSONB solo contiene IDs, títulos, tipo, extensión, temporada/episodio y contentKey validados; excluye raw URLs, direct_source y secretos. Se guarda al consultar, nunca descarga masiva. TTL 60 s en RAM/PG: antes de vencer se reutiliza, después se consulta proveedor; **no hay fallback stale**. Proveedor caído devuelve 502/504 y no marca datos viejos como actuales. El contenido permanece almacenado hasta refrescar o eliminar la fuente.

Antes de playback de fuente hidratada o con validación de cuenta anterior a 60 s se revalida estado/max_connections del proveedor. IDs/extensiones provienen del catálogo saneado, nunca URL cliente. Logout conserva registro/catálogo cifrado, pero revoca accesos activos; DELETE /sources elimina persistencia y cache para que no reaparezcan. Un reinicio no restaura streams ni cookies expiradas, solo permite recuperar la configuración propia.

Migración aditiva 002 mediante walactv migrate con locking y todas las versiones ordenadas. Recuperación exige backup PostgreSQL **y** la clave externa original; no se genera otra clave sobre una instalación existente ni se hace rekey silencioso.
