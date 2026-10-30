# Contrato de catálogo persistente y descubrimiento

Cambio `improve-iptv-discovery`, 2026-10-28. Complementa `docs/api-contract.md`; sustituye la política de snapshots de navegación de 60 s para live/movie/series. Fuentes y autenticación existentes se conservan. No contiene configuración privada.

## Endpoints

Base `/api/v1`; cookie, propietario, CSRF/Origin de mutaciones y errores saneados existentes. Una fuente ajena devuelve 404 en todos los endpoints.

| Método/ruta | Resultado |
|---|---|
| GET `/sources/{id}/catalog` | 200 `CatalogStatus` sin acceder al proveedor |
| POST `/sources/{id}/catalog/refresh` | 202 `CatalogStatus`; inicia o reutiliza un único trabajo de sincronización por fuente |
| GET `/sources/{id}/categories?type=live\|movie\|series` | 200 `{categories:[{id,name}]}` desde generación activa |
| GET `/sources/{id}/languages?type=live\|movie\|series` | 200 `{languages:[{id,name}]}` desde generación activa |
| GET `/sources/{id}/items?type=...&categoryId=...&language=...&search=...&page=1&pageSize=40` | 200 `{items:[Item],total,page,pageSize}` desde PostgreSQL |
| GET `/sources/{id}/images/{kind}/{itemId}` | Imagen binaria autorizada; kind `live\|movie\|series`; URL proveedor nunca aceptada como parámetro |

`CatalogStatus={status:"empty|syncing|ready|error",updatedAt?:ISO8601,errorCode?:string,counts:{live:number,movie:number,series:number}}`. Durante `syncing/error`, `updatedAt` y counts describen la generación anterior visible; sin generación counts=0. Interfaz muestra «Actualizando», «Catálogo guardado · fecha» y aviso de fallo/reintento; datos guardados no se presentan como frescos si falla el proveedor. GET sin generación devuelve arrays vacíos; no fuerza red ni devuelve error genérico. Al crear/restaurar una fuente sin generación, UI solicita una sincronización una vez; POST es idempotente mientras haya trabajo.

`Item={id,title,kind,containerExtension?,contentKey?,categoryId,language,imageUrl?}`. `language` es código estable inferido de etiquetas explícitas del proveedor (`es`, `en`, `fr`, `de`, `it`, `pt`, etc.); desconocido `und`. `imageUrl`, si disponible, es exclusivamente ruta relativa propia. Sin imagen o al fallar la carga, UI muestra fallback local sin reintentos infinitos. `Language={id,name}` contiene solo idiomas presentes; `und` se muestra «Desconocido». Idioma inferido de nombre/categoría no acredita pista de audio.

Omitir categoryId o language significa todas las categorías o idiomas, respectivamente. UI nunca envía ID sintético «Todas». `search` busca coincidencia literal sin distinguir mayúsculas en el nombre de todo el tipo seleccionado, combinada por AND con categoría/idioma y antes de paginar. Caracteres `%`/`_` no son comodines de usuario. Orden determinista título+ID. page>=1, pageSize 1..100, search máximo 150 caracteres; entradas inválidas 400. Cambio de filtro reinicia página; respuesta vieja de petición cancelada no sustituye resultados actuales.

## Sincronización y persistencia

Trabajo asíncrono por fuente; proveedor se consulta secuencialmente mediante acciones globales de categorías e items para los tres tipos, sin enviar category_id cuando se pide el catálogo completo. Como máximo una request de catálogo simultánea por cuenta; no se abren streams ni leases para sincronizar. Deduplicar clicks/pestañas. No ejecutar sincronización periódica agresiva ni descargar episodios de cada serie; episodios siguen bajo demanda.

Límites de importación: 256 MiB por respuesta de catálogo, 500.000 elementos por tipo, 120 s por petición de catálogo y 10 minutos por trabajo. La respuesta se procesa por partes; estos límites se ajustaron tras comprobar un catálogo real con más de 210.000 películas. Límite/error/cancelación no publica datos incompletos. Nuevos items/categorías se escriben en generación de staging; publicación activa y contadores en una transacción solo tras éxito de los tres tipos. Borrado/reemplazo de fuente cancela e invalida trabajo; FK/generación impiden resurrección. Reinicio recupera generación activa y transforma sync interrumpida en estado reintentable; no inventa éxito. Fuente persistida sin catálogo no se pierde si proveedor falla.

PostgreSQL consulta solamente generación activa propia con índices por fuente/generación/tipo/categoría/idioma y búsqueda normalizada. Migración aditiva 003. Datos públicos saneados en columnas; originales de imágenes en ciphertext AES-GCM con clave existente y AAD propietario+fuente+tipo+ID. No guardar respuestas raw, URL privada, direct_source ni extensiones inferidas. Reproducción reconstruye URL validada usando metadatos de generación activa y revalida cuenta según contrato anterior. Catálogo offline no acredita permiso de reproducción.

## Imágenes

Mapear `stream_icon` de live y `stream_icon`/`cover` de VOD a ciphertext; nunca exponer URL original al navegador. Validar HTTP(S), DNS público y cada redirección/conexión; rechazar IP privada/link-local/metadata, userinfo y fragmentos. Un CDN público de imagen no requiere ampliar la allowlist de streams: utiliza cliente de imagen separado con egress público estricto. No reenviar cookies, CSRF, Authorization ni cabeceras cliente a terceros. TLS verifica host original. No endpoint de proxy URL libre.

Máximo 5 MiB y 10 s por imagen, tres redirecciones; MIME permitido JPEG/PNG/WebP/GIF con comprobación de firma. No SVG/HTML. Caché acotada en RAM por identidad/generación, respuesta `Cache-Control: private,max-age=3600`, `X-Content-Type-Options: nosniff`, sin propagar cookies o Location del proveedor. Fallo 404/502 genérico y fallback UI. Autorización siempre antes de caché.

## Películas, series y enriquecimiento

Rutas dedicadas y carga bajo demanda para Directos/Películas/Series. Carátulas, filtros y búsqueda de cada tipo; ficha con título, categoría/idioma inferidos y datos reales disponibles del proveedor. Series despliegan temporadas/episodios y películas reproducen usando tipo/ID de catálogo; no fabricar metadatos ni sustituir proveedor de reproducción. Una ficha puede mostrar solo datos disponibles sin bloquear reproducción.

La ficha puede consultar TMDB oficial en `es-ES` solo cuando el usuario abre explícitamente una película/serie propia; una temporada se consulta únicamente al seleccionarla. Antes de tocar caché o red, Go autoriza propietario, fuente, tipo, ID y generación activa del catálogo. La búsqueda transmite título normalizado y año conocido; nunca ID IPTV, categoría, URL, credenciales ni identidad de usuario. Sin año o ID externo estructurado válido no se busca y la ficha conserva datos IPTV. Un candidato se asocia solo con título exacto normalizado y año coincidente. Importación, listados, búsqueda, arranque y playback no consultan TMDB. Los episodios e imágenes IPTV mantienen autoridad sobre reproducción/disponibilidad; TMDB solo aporta texto y arte.

Las rutas aditivas están descritas en `docs/api-contract.md`. La caché canónica contiene campos TMDB públicos y localizados, separada de decisiones privadas por propietario/fuente/item/huella. No existe `tmdb_id` o `imdb_id` en el catálogo histórico 003; 004 conserva IDs/año estructurados solo si una respuesta de catálogo ya los incluye. La resolución por título necesita año conocido. Abrir una ficha fría puede tardar hasta el límite de red configurado; una copia stale (máximo 30 días) se entrega inmediatamente y se refresca una vez. No se realiza barrido global ni precarga de temporadas.

## Operación

No nuevas claves obligatorias ni servicios Compose; PostgreSQL externo en producción y local aislado actuales, API sigue una réplica. Migración 003 se ejecuta con job `walactv migrate` bajo advisory lock antes de API. Backup privado local antes de migración, no volcar secretos ni Compose resuelto. Rollback de imagen conserva nuevas tablas que la versión anterior ignora; no hace down destructivo. `IPTV_ENCRYPTION_KEY` original sigue necesaria para fuente e imágenes.
