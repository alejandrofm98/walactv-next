# Investigación: metadatos de películas y series en español

Investigación realizada el 2026-10-29 para decidir cómo mostrar fichas en español sin descargar la base global de películas y series. Las pruebas de servicios públicos son muestras puntuales, no una prueba de rendimiento ni un SLA.

## Resumen de la recomendación

No existe una API pública documentada de Cinemeta que podamos configurar con `language=es` y usar como backend propio. Stremio anunció “Enhanced Cinemeta” con traducción al idioma del usuario, pero se configura en la cuenta y las aplicaciones Stremio; no se documenta como API externa para WalacTV ([anuncio oficial](https://blog.stremio.com/stremio-supporters-a-way-to-sustain-our-development/)). Cinemeta sigue siendo útil dentro de Stremio, pero sus fichas públicas no garantizan español.

La opción más práctica para WalacTV es **TMDB localizado bajo demanda, con caché persistente propia**:

1. Importar y guardar solo el catálogo real del usuario IPTV, como ya hace WalacTV.
2. En las páginas de catálogo, mostrar primero título, categoría e imagen del proveedor; no esperar a que TMDB conteste.
3. Al abrir un título, resolver su identidad en TMDB y pedir la ficha en `es-ES` (con opción futura `es-MX`). Si la ficha está guardada, responder desde PostgreSQL; si no, traerla una vez, mostrarla y guardarla.
4. Pedir temporadas y episodios al abrir cada temporada, no recorrer cada serie completa durante la sincronización.
5. Hacer búsquedas/mapeos de forma conservadora y guardar también el resultado “sin coincidencia” durante un tiempo, para no repetir consultas inútiles.

Esto evita recorrer las 210.381 películas y 45.620 series del catálogo importado. **No elimina la primera consulta de red de un título frío**: si se quiere que incluso la primera visita sea inmediata y no haya llamadas en tiempo real, hay que precargar títulos elegidos por uso/favoritos o aceptar fichas incompletas mientras se rellenan en segundo plano. Ningún servicio fiable ofrece simultáneamente metadatos en español para todo el universo, cero latencia fría, cero importación y límites/servicio garantizados.

## Qué hacen servicios y aplicaciones comparables

### TMDB

TMDB permite pedir resultados en un idioma concreto y usar búsquedas para resolver títulos; documenta idiomas y el flujo para encontrar datos en su [guía de idiomas](https://developer.themoviedb.org/docs/languages) y [guía de búsqueda](https://developer.themoviedb.org/docs/finding-data). Para nuestras fichas de película/serie, el backend puede usar el ID TMDB ya conocido o buscar por nombre y año, y luego pedir detalles/localizaciones.

TMDB no publica un cupo fijo garantizado: su documentación indica límites superiores aproximados en torno a 40 peticiones por segundo, sujetos a cambio, y pide respetar respuestas `429` ([límites oficiales](https://developer.themoviedb.org/docs/rate-limiting)). Ese número no debe tratarse como capacidad reservada para WalacTV. Su export diario tampoco resuelve el problema: contiene IDs y algunos campos de alto nivel, no las fichas completas traducidas, pósteres y sinopsis ([contenido de los exports](https://developer.themoviedb.org/docs/daily-id-exports)). El uso no comercial requiere atribución y el comercial requiere licencia ([FAQ oficial](https://developer.themoviedb.org/docs/faq)); habrá que decidirlo antes de ofrecer el servicio públicamente.

### Nuvio y Stremio

Nuvio tiene un cliente TMDB con idioma configurable. Su implementación consultada conserva resultados en caché en memoria y reutiliza llamadas en curso; enriquece lo que está mostrando, incluyendo solicitudes paralelas de datos relacionados, en vez de descargar todo TMDB. Se puede revisar en el [servicio TMDB de Nuvio](https://github.com/NuvioMedia/NuvioTV/blob/dev/app/src/main/java/com/nuvio/tv/core/tmdb/TmdbMetadataService.kt) y el [enriquecedor de fichas](https://github.com/NuvioMedia/NuvioTV/blob/dev/app/src/main/java/com/nuvio/tv/core/tmdb/MetaPreviewEnricher.kt). Es evidencia del patrón de aplicación, no garantía de que el código o su política de caché no cambien.

Stremio separa catálogos, fichas (`meta`) y fuentes de reproducción (`stream`) en su [protocolo de add-ons](https://stremio.github.io/stremio-addon-sdk/protocol.html). Un add-on TMDB compatible con Stremio puede devolver fichas en distintos idiomas y es una buena prueba de concepto. Probé el add-on público comunitario [`mrcanelas/tmdb-addon`](https://github.com/mrcanelas/tmdb-addon) en una instancia pública: el manifiesto y fichas de *Matrix* y *La casa de papel* devolvieron textos en español. Las llamadas frías observadas fueron del orden de 0,6 s; una página de catálogo rondó 1,1 s. Hubo respuestas posteriores más rápidas, pero las cabeceras observadas no prueban un caché CDN estable. Son una muestra pequeña en una fecha concreta, no promesa de latencia.

La instancia pública de ElfHosted no es apropiada como backend de enriquecimiento automatizado: sus condiciones dicen que es compartida, no tiene compromiso de disponibilidad, puede limitarse o retirarse y prohíbe uso automatizado, masivo o comercial ([condiciones de instancias públicas](https://docs.elfhosted.com/legal/public-instances/)). Por tanto, no se debe hacer un barrido de nuestro catálogo contra esa instancia. Una instancia privada propia o llamar a TMDB directamente elimina esa dependencia pública; no elimina el trabajo de caché ni las llamadas frías. El add-on privado AIOMetadata está diseñado para agregación y caché de metadatos, pero añade otro servicio, proveedor y coste/operación ([documentación de AIOMetadata](https://docs.elfhosted.com/app/aiometadata/)); conviene evaluarlo como producto independiente, no asumirlo como una API pública gratuita estable.

### Kodi y Jellyfin

Los gestores de bibliotecas tradicionales escanean lo que el usuario ya tiene localmente y guardan los resultados asociados a cada archivo. Kodi usa scrapers y permite elegir proveedor/idioma; Jellyfin configura el idioma del proveedor y descarga metadatos durante el escaneo de la biblioteca ([Kodi scrapers](https://kodi.wiki/view/Scrapers), [asistente de Jellyfin](https://jellyfin.org/docs/general/post-install/setup-wizard/)). Ese modelo sí puede generar llamadas para cada elemento, pero la colección es del usuario, se hace como trabajo de importación/actualización y el resultado queda local. No implica que Stremio o Nuvio predescarguen información para toda la base mundial.

## Pruebas de servicios públicos

El 2026-10-28 hice peticiones públicas de prueba usando IDs conocidos de títulos; no envié la URL IPTV ni datos de usuario.

| Servicio | Resultado de muestra | Lectura |
|---|---|---|
| TMDB Stremio addon público, idioma `es-ES` | Manifiesto ~208 ms; ficha de película ~644 ms y segunda petición ~358 ms; serie ~642/347 ms; catálogo de 20 títulos ~1,1 s | Devuelve títulos y sinopsis españoles. La muestra confirma localización, no estabilidad ni SLA. |
| Cinemeta público | *The Matrix* en inglés, ~72 ms en respuesta marcada CDN `HIT` | Rápido en esa lectura, pero no resuelve por sí solo el requisito de español. |
| Instancia oficial/comunitaria de TMDB addon consultada | Una respuesta agotó ~12 s con 504 | Señal de que las instancias públicas varían; una sola petición no permite evaluar su fiabilidad. |

Las respuestas y tiempos externos cambian según caché, carga, región y estado de los proveedores. Antes de adoptar un host compartido habría que repetir un benchmark controlado desde nuestro entorno y revisar sus condiciones; las cifras anteriores no son un compromiso.

## Encaje con WalacTV actual

WalacTV guarda el catálogo IPTV por usuario en PostgreSQL y ya carga **45.669 canales, 210.381 películas y 45.620 series** de la fuente probada. Las entradas incluyen ID IPTV, nombre, categoría, idioma inferido, extensión e imagen del proveedor; no conservan hoy un `tmdb_id` o `imdb_id` externo. El modelo y el contrato están descritos en [`sources.go`](../apps/api/internal/api/sources.go), [`discovery_catalog.go`](../apps/api/internal/api/discovery_catalog.go) y [`catalog-contract.md`](catalog-contract.md).

El catálogo guardado responde búsqueda/filtros sin volver a consultar la fuente. Los episodios se consultan al pedirlos. Es una base adecuada: añadir una caché metadata por obra no exige modificar la fuente privada ni descargar el catálogo completo de TMDB.

El riesgo mayor no es la API, sino la **resolución de identidad**. El ID IPTV no tiene por qué coincidir con ningún ID de TMDB. La búsqueda por título puede confundir año, remake, idiomas, cortes y versiones de calidad. Conviene:

- Priorizar un IMDb/TMDB ID si el proveedor lo aporta en alguna respuesta; guardar ese enlace al catálogo.
- De lo contrario buscar por título normalizado y año cuando se pueda inferir, y guardar la decisión para ese ID IPTV.
- En coincidencias dudosas, no enlazar automáticamente; permitir elegir entre candidatos o dejar la ficha IPTV básica.
- Si varios enlaces/calidades del proveedor corresponden a la misma obra, hacer que compartan una ficha canónica, aunque mantengan streams IPTV independientes.
- No enviar el nombre completo de categorías/grupos privados a un tercero; mandar solo la consulta mínima del título cuando el usuario abre la ficha.

## Diseño recomendado

### Enfriado de caché y experiencia

- Caché persistente en PostgreSQL por `(proveedor de metadatos, tipo, ID canónico, idioma)`; almacenar la respuesta normalizada y su `fetched_at`/caducidad. No duplicar ficha por cada calidad del mismo contenido.
- Para identidad sin resolver, clave por `(source_id, kind, IPTV_item_id)` y guardar el candidato/estado de resolución, evitando repetir búsquedas.
- Responder de inmediato con la ficha IPTV disponible. Si la metadata está fría, el backend consulta TMDB una vez, guarda resultado y el frontend la actualiza cuando llega. Una alternativa UX es el skeleton, pero no bloquear la navegación ni reproducción.
- Deduplicar llamadas simultáneas por ID (`singleflight`) y limitar concurrencia. En `429`, aplicar `Retry-After`/backoff; en errores de red, conservar el título y póster del proveedor. El fallback no debe convertir fallos en fichas vacías almacenadas indefinidamente.
- TTL mayor para sinopsis/créditos y más corto o sin caché para disponibilidad de proveedores. Usar stale-while-revalidate: mostrar una ficha caducada mientras se renueva en segundo plano.
- Precarga pequeña y de prioridad: primera página visible, favoritos y continuar viendo, con cupo bajo por usuario; añadir trabajos de precarga solo si la medición muestra beneficio. No recorrer todas las películas/series ni todas las temporadas.
- Configurar idioma por perfil con `es-ES` como valor inicial; permitir cambiarlo sin duplicar toda la ficha. El texto localizado cambia por idioma, mientras ID y arte pueden compartirse cuando la fuente no cambia.

### Servicio externo

Para la primera versión productiva, preferir **TMDB oficial desde el backend Go**, con su clave solo en variable secreta del servidor y atribución en la UI. Esto da contrato/API y control de reintentos/caché propios, y evita CORS y dependencia de un endpoint público de terceros. Si se quiere conservar el protocolo Stremio, se puede evaluar un add-on TMDB autoalojado detrás de un adaptador Go; en ese caso el adaptador/cache siguen siendo responsabilidad de WalacTV. No exponer clave TMDB al navegador.

No adoptar Cinemeta como fuente principal española. No depender del addon público compartido como servicio crítico. No copiar globalmente la base TMDB. Si el uso pasara a ser comercial, resolver licencia/atribución con TMDB antes de hacerlo público.

## Conclusión

La alternativa a “recorrer toda la API TMDB” no es otra API mágica que ya conozca cada ID IPTV; es consultar y guardar únicamente los títulos que aparecen en la experiencia del usuario. En WalacTV, la decisión práctica es **catálogo privado persistido + identificación gradual + ficha TMDB `es-ES` a demanda + caché compartida persistente + respuesta IPTV de respaldo**. El coste de red ocurre en la primera consulta de cada obra; desde entonces la caché evita repetirla hasta refresco. La precarga se limita a lo que cada usuario realmente navega.

Fuentes oficiales y primarias enlazadas arriba. Esta nota es investigación; no modifica API, esquema PostgreSQL, Compose, secretos ni proveedor configurado.

## Descubrimiento restringido por IMDb · 2026-11-01

La decisión anterior sigue vigente para las fichas: TMDB oficial en `es-ES` se consulta bajo demanda y Cinemeta no sustituye sus textos localizados. El cambio `add-cinemeta-iptv-discovery` usa Cinemeta únicamente cuando la persona activa explícitamente el modo de descubrimiento en Películas o Series. Su catálogo Popular (`top`) aporta orden y géneros; búsqueda es enviada solo cuando se confirma. No se piden imágenes ni streams a Cinemeta.

La disponibilidad se verifica en Go con `imdb_id` estructurado del catálogo IPTV activo, el mismo tipo y año cuando ambos están disponibles. Títulos semejantes no se enlazan. Se conservan las versiones IPTV (calidad, categoría, idioma aproximado, imagen del proveedor e identidad de reproducción); idiomas y categorías privados nunca se envían a Cinemeta. Los valores de género del manifiesto se muestran con etiquetas españolas locales. Cinemeta no garantiza el idioma ni la salud del stream.

El adaptador llama al catálogo Stremio con la ruta `/catalog/{type}/top/{extraArgs}.json`; género, búsqueda y `skip` van en `extraArgs` separados por `&`. Cada valor recibe percent-encoding de componente (equivalente a `QueryEscape`, con espacios como `%20` en lugar de `+`) antes de unir los pares, para que caracteres `&`, `+` y `=` dentro de una búsqueda no creen argumentos adicionales. Se envían en el path, no como parámetros de query. Usa HTTPS con hosts fijos, límite de 5 s y 2 MiB, dos solicitudes externas simultáneas, separación global de 4 solicitudes/s, enfriamientos temporales ante 429/5xx y una caché pública en memoria de 15 minutos, hasta 128 lotes/16 MiB. Cada acción recorre como máximo tres lotes externos de 100; la respuesta contiene hasta 40 versiones. Los cursores son privados, efímeros y vinculados a usuario, sesión, fuente, generación y filtros. Una API reiniciada o un cambio/expiración de contexto exige recomenzar. La búsqueda explicitada sale del backend; no se envían identificadores IPTV ni historial. No hay caché privada compartida, total Cinemeta presentado como total IPTV, token Cinemeta, ni petición durante importación, listado normal, reproducción o arranque.

La investigación histórica de esta nota describe que las filas de la importación original carecían de IDs externos. Desde la migración 004, `source_catalog_entries` contiene `imdb_id`, `tmdb_id` y `release_year` opcionales; Cinemeta usa solo el IMDb estructurado existente. Este cambio no rellena esos campos, no añade una migración ni intenta resolver títulos del catálogo mediante búsquedas masivas.
