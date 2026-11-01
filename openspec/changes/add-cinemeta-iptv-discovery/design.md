# Design

## Context

Ver motivación en `proposal.md`. La API Go guarda entradas por fuente/generación/tipo en `source_catalog_entries`; migración 004 ya añadió `imdb_id`, `tmdb_id` y `release_year`. `catalogSource` autoriza fuentes y el catálogo consulta solo `active_generation`. TMDB tiene servicio independiente de fichas `es-ES`, resoluciones privadas y caché persistente. Angular conserva filtros en `CatalogViewStateService` y usa rutas propias para fichas/playback.

Los deltas existentes de descubrimiento impiden enviar títulos privados automáticamente y exigen identidad conservadora. Este cambio mantiene esa regla: solo la búsqueda escrita explícitamente en el modo Cinemeta sale del backend, nunca una búsqueda masiva por títulos importados. Ver `docs/cinemeta-research.md`: Cinemeta no garantiza español; se usa para clasificación y orden, no para reemplazar fichas TMDB.

Referencias públicas verificadas el 2026-11-01: [manifiesto Cinemeta](https://v3-cinemeta.strem.io/manifest.json), [contrato de catálogo Stremio](https://stremio.github.io/stremio-addon-sdk/api/requests/defineCatalogHandler.html) y [formato de manifiesto](https://stremio.github.io/stremio-addon-sdk/api/responses/manifest.html). El manifiesto identifica tipos movie/series e IDs IMDb `tt`; el catálogo `top` declara `search`, `genre` y `skip`. El protocolo usa normalmente lotes de 100. No se presupone disponibilidad de streams ni idioma español.

## Goals / Non-Goals

**Goals:** introducir un contrato aditivo acotado, composición de filtros con autorización previa e integración con navegación vigente.

**Non-Goals:** catálogo externo completo descargado, búsqueda de identidad de cada título IPTV, lectura masiva de fichas TMDB, favoritos Cinemeta, ampliación de disponibilidad, nuevos controles de Directos o dependencia externa para salud/playback. No se añaden filtros de año/valoración ni feeds distintos de Popular en esta fase.

## Decisions

### Identidad y disponibilidad

Usar solo `imdb_id` estructurado válido del catálogo activo, validado con el patrón existente, junto con tipo y ausencia de contradicción de año conocido. Títulos traducidos no invalidan por sí solos un ID exacto. No escribir asociaciones nuevas ni reutilizar un ID TMDB como IMDb. Cada consulta toma un conjunto de IDs externos y resuelve todas las versiones IPTV en una consulta SQL parametrizada por fuente, generación, tipo, IMDb y filtros privados. Orden por posición externa y después ID IPTV estable. No hacer una consulta por obra ni cargar todo el catálogo privado en memoria.

Alternativa descartada: fuzzy match o buscar cada título en TMDB/Cinemeta; produciría falsos positivos, tráfico proporcional al catálogo y contradicción con el enriquecimiento bajo demanda. Fuentes M3U o catálogos sin IMDb pueden tener cobertura nula: mostrarlo expresamente y conservar el catálogo completo. La implementación actual de fuentes se comprueba sin ampliar importadores para intentar extraer IDs de texto libre.

Una serie del catálogo activo es una entrada de descubrimiento, no un episodio automáticamente reproducible. La ficha obtiene episodios mediante el flujo IPTV existente; sin episodios disponibles no ofrece reproducir. No se hacen probes de vídeo ni peticiones de episodios por cada tarjeta: presencia IPTV es la definición operativa de disponibilidad, no salud instantánea del stream.

### Contrato aditivo

Añadir `GET /api/v1/sources/{id}/discovery` autenticado. Parámetros: `type=movie|series`, `genre`, `search` explícito, `categoryId`, `language` y `cursor` opcional. Popular es el único catálogo inicial. Respuesta: `items` de identidad IPTV, `nextCursor`, `hasMore`, `partial`, `status` y `coverage=imdb-only`; no total global. `status` distingue `ready`, `unavailable` y `restart_required`. HTTP 400 para entrada inválida, 404 fuente ajena/ausente, 401 sesión expirada; saturación/enfriamiento usan estado temporal saneado sin filtrar errores raw.

Autorizar antes de usar cursores o caché. Cursores opacos aleatorios en memoria, vinculados a usuario/sesión/fuente/generación/consulta, TTL de 10 minutos, máximo global 128 contextos y 8 por usuario con expulsión LRU. Una continuación inválida por TTL, expulsión, reinicio o cambio de generación devuelve `restart_required`, no una lista vacía definitiva. Revocación de sesión/fuente invalida contextos y cancela trabajo; comprobar autorización/generación también antes de entregar respuesta.

### Continuidad entre páginas dispersas

Cada contexto mantiene desplazamiento externo, lote externo pendiente con offset interno, versiones ya emitidas y generación. Respuesta de hasta 40 versiones; consumir como máximo tres páginas externas nuevas por petición, detenerse antes si se llena la página y conservar todas las coincidencias restantes para la continuación. `hasMore` depende del buffer o continuidad externa, no de la cantidad de matches. Tres páginas vacías privadas con continuación externa producen `partial=true` y cursor para seguir. Un lote externo final corto/vacío según protocolo determina fin solo después de consumir el buffer.

Para acotar duplicados y memoria, máximo 2.000 versiones emitidas y 1 MiB de estado por contexto; al agotarse, devolver `partial=true`, `restart_required` y explicación de límite de exploración, sin afirmar que terminó el catálogo. Deduplicar por tipo e ID IPTV. Consumir el cursor de forma idempotente: solicitudes duplicadas de una misma página devuelven la respuesta ya producida mientras dure el contexto; un fallo de red no avanza el desplazamiento. Serializar avances por contexto; máximo dos peticiones activas por usuario, rechazar excesos sin encolado ilimitado.

Alternativa descartada: aplicar page/pageSize directamente al resultado externo y luego ocultar ausentes; permitiría saltar versiones o confundir una página dispersa con el final. Tampoco se anuncia un total exacto: exigiría recorrer un catálogo externo potencialmente enorme.

### Adaptador, límites y caché

Usar HTTP cliente inyectable para pruebas. HTTPS a `v3-cinemeta.strem.io`; si necesita redirect de catálogo permitir únicamente `cinemeta-catalogs.strem.io`, máximo dos saltos, sin redirects a HTTP, IPs o hosts aportados por usuario. No construir red desde imágenes/URLs del payload. Géneros permitidos por tabla explícita derivada del manifiesto, con etiquetas españolas y valores originales; sin depender de consultar manifiesto al arrancar. Revalidar contrato mediante fixture pública saneada y comprobación manual independiente.

Aplicar límites de la spec: 5 s/2 MiB por petición, 2 simultáneas/4 por segundo globales, 10 acciones por minuto por usuario, tres páginas por respuesta. Contexto de petición completo máximo 16 segundos, cancelación propagada. Cola máxima 16; exceso devuelve estado temporal. Respetar Retry-After; enfriamientos en memoria, no persistencia nueva. Reiniciar API elimina caché y enfriamientos; no emitir tráfico automático al volver. No reintentar automáticamente.

Caché pública en memoria, 15 minutos, máximo 128 lotes y 16 MiB LRU; solo datos Cinemeta normalizados, nunca resultados intersectados ni datos IPTV. Clave por tipo/género/búsqueda/desplazamiento; deduplicar peticiones iguales. No registrar consultas completas, respuestas raw ni credenciales. Respuestas del endpoint privado con `Cache-Control: private, no-store`. No usar service worker o caché pública para resultados privados.

### UI y convivencia con cambios activos

Añadir modo de descubrimiento en las secciones existentes de Películas/Series, manteniendo filtros IPTV independientes del estado Cinemeta. Género público y categoría privada son controles distintos; idioma mantiene la advertencia de aproximación IPTV. La búsqueda solo se envía al confirmar explícitamente, con aviso de consulta externa. Mostrar hasta 40 versiones, navegación Anterior desde páginas ya visitadas y Continuar por cursor; admitir página vacía con acción «Seguir buscando coincidencias». Atribuir clasificación a Cinemeta y mantener los datos/artwork IPTV de tarjetas; no descargar imágenes arbitrarias de Cinemeta.

Ampliar el estado de navegación con modo, género, búsqueda y páginas/cursor por usuario/fuente. Al volver, revalidar generación; reiniciar con mensaje si caducó. Cancelar solicitudes y descartar respuestas tardías. Mantener las fichas y los detalles TMDB `es-ES` sin cambios; no llamar TMDB al listar, filtrar o paginar. Preservar ediciones concurrentes de biblioteca/player y el trabajo de otras propuestas.

## Risks / Trade-offs

- [Pocos IMDb en proveedores/M3U] → cobertura parcial explícita y acceso permanente al catálogo completo; no inventar asociaciones.
- [Páginas externas poco densas] → continuar por cursor, límites por acción y distinción entre parcial y fin.
- [Filtros combinados no respetados por proveedor] → fixtures comprueban el contrato; si una combinación deja de ser válida devolver indisponibilidad, no resultados sin filtrar.
- [Catálogos grandes] → consulta por lote, sin N+1; medir con fixture de 210.381 películas y 45.620 series. No añadir índice/migración preventiva; si las mediciones bloquean, devolver evidencia y proponer ajuste acotado antes de ampliar esquema.
- [Datos externos no españoles] → etiquetas locales, atribución Cinemeta y detalles TMDB bajo demanda; no mostrar inglés como sinopsis localizada.
- [Cursores efímeros y catálogo externo cambiante] → reinicio honesto y deduplicación por versión; no prometer instantánea global del catálogo Cinemeta.

## Migration Plan

No hay migración de esquema: se reutilizan los campos de 004 sin backfill ni escritura de metadata Cinemeta. El job `walactv migrate` conserva sus versiones existentes y se ejecuta antes de API como ahora. No cambian Compose, puertos, servicios, volúmenes, variables ni secretos; Cinemeta pública no exige token. `TMDB_API_TOKEN` sigue opcional y exclusivo del backend; `API_REPLICAS=1` se conserva por coordinación actual en memoria. PostgreSQL local aislada y PostgreSQL productiva externa mantienen sus ciclos existentes.

Verificar Go/Angular y Compose `config --quiet` sin resolver/imprimir secretos, salud sin acceso Cinemeta y regresión de catálogo/playback. Un rollback de imagen elimina cursores/caché efímeros y restaura navegación anterior; no revierte ni modifica datos PostgreSQL. La fase de implementación no autoriza push/despliegue: `origin/main` provoca Auto Deploy de producción según `docs/deployment.md`, revalidado el 2026-11-01. Cualquier publicación exige autorización y destino real comprobado por el coordinador.
