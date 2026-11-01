# Design

## Context

La primera implementación local añadió rutas técnicas `/sources/:sourceId/...`, consulta puntual de item, fichas vinculadas al router y lifecycle de reproducción. Esta revisión modifica su presentación sin duplicar el flujo. `sources.user_id` es UNIQUE en 002 y `createSource` sustituye la fila previa con ID aleatorio nuevo: como máximo existe una fuente activa por usuario. `GET /api/v1/sources` permite resolverla desde sesión. El catálogo sigue usando `(source_id, generación activa, tipo, id)` y Nginx ya tiene fallback SPA.

## Goals / Non-Goals

Permitir selección inequívoca y recuperación directa con sesión válida, mantener navegación por historial y preservar controles, accesibilidad, filtros y lifecycle actuales. Los enlaces son privados y vinculados a la fuente persistida de una cuenta. No se promete resolver el mismo ID de proveedor en otra cuenta ni después de borrar/recrear una fuente; no se hacen públicos metadatos o medios.

## Decisions

### 1. Rutas legibles y origen compacto

| Destino | Ruta canónica |
| --- | --- |
| Canal y su reproductor | `/directo/:slug-:itemId?o=:origen` |
| Ficha de película | `/peliculas/:slug-:itemId?o=:origen` |
| Reproductor de película | `/peliculas/:slug-:itemId/ver?o=:origen` |
| Ficha de serie | `/series/:slug-:seriesId?o=:origen` |
| Episodio y su reproductor | `/series/:slug-:seriesId/episodios/:episodeLabel-:episodeId?o=:origen` |

La ficha de serie admite `&temporada=N`; el episodio conserva `&temporada=N` con su temporada real para el regreso. Orden canónico de query: `o` y después `temporada`. La temporada ausente en ficha elige la primera disponible sin exigir añadirla a la URL; una presente pero inválida se normaliza por reemplazo. El número nunca identifica un episodio: lo hacen los IDs de serie y episodio, comprobando su pertenencia.

El contrato existente exige IDs de 1–32 dígitos (`numericID`). Cada segmento compuesto se analiza por su último guion: sufijo ID decimal como string, prefijo slug validado (o descriptor tNeM en episodio); conservar ceros iniciales y no convertir el ID a número. Así `titulo-2026-123` identifica exactamente ID `123`, sin búsquedas ambiguas. El descriptor de episodio es `t{season}e{episode}` (ejemplo `t1e2-456`); ambos números proceden del episodio resuelto por ID, son decorativos y se corrigen mediante reemplazo si difieren. No resolver episodios por esos números ni exigir que sean únicos. La identidad de reproducción es `(source.id, tipo, itemId, seriesId)`; cambios decorativos de slug o query no reinician playback.

`origen = base64url(primeros 12 bytes de SHA-256(UTF-8(source.id)))`, sin padding: exactamente 16 caracteres `[A-Za-z0-9_-]`. Se calcula con Web Crypto usando el ID que ya devuelve `/sources`; no usa secretos, accountKey ni títulos y no autoriza acceso. Es una referencia de coherencia de 96 bits, no un identificador de base de datos. Conservarla obligatoriamente en todos los enlaces nuevos permite detectar sustitución/recreación de fuente sin mostrar su ID largo, añadir tablas, variables o modificar contratos API. Un origen diferente, ausente o malformado impide resolver/reproducir automáticamente; se ofrece ir al catálogo para generar un enlace actual. Nunca completar uno ausente con la fuente actual. Se eligen 16 caracteres frente a referencias más cortas para conservar 96 bits sin estado adicional ni detección de colisiones en BD; no reemplaza ninguna autorización Go.

Sin referencia de origen, dos fuentes sucesivas pueden tener el mismo ID y título: ni la fuente única por usuario ni validar el slug permiten distinguirlas. No eliminar `o` al normalizar una URL o conservar un returnUrl. La sustitución cambia source.id aunque se registre otra vez la misma cuenta; los enlaces anteriores quedan deliberadamente obsoletos.

### 1.1. Slug decorativo con gramática estricta

El slug sirve para leer el título; los IDs y el origen determinan qué contenido es. Generarlo desde el título IPTV saneado, no desde metadata TMDB variable: para directo aplicar primero la limpieza visible de marcadores `#`; después normalización Unicode NFKD, eliminación de marcas diacríticas, minúsculas, sustitución de grupos ajenos a `[a-z0-9]` por `-`, recorte de guiones y límite de 80 caracteres sin guion final. Fallback `canal`, `pelicula`, `serie` o `episodio` si queda vacío. Mismo algoritmo central para todas las acciones y redirecciones.

Validar longitud 1–80 y gramática `[a-z0-9]+(?:-[a-z0-9]+)*` en segmentos de slug; rechazar escapes malformados, separadores codificados, controles y segmentos vacíos. No interpretar nunca slug como HTML, URL ni consulta. Un slug gramaticalmente válido pero distinto al actual (renombrado o manipulado) se reemplaza por el canónico tras resolver IDs y origen, sin entrada de historial extra ni POST previo. Slugs inválidos presentan destino inválido/regreso seguro y no abren medios. Dos títulos iguales o normalizados al mismo slug no colisionan porque sus IDs siguen separados. Un slug correcto nunca compensa un ID/origen incorrecto.

Los títulos pasan a formar parte del historial y los enlaces por petición del usuario; no incluir credenciales, URL IPTV, tokens de sesión/medios ni IDs temporales de playback. Mantener `Referrer-Policy: no-referrer` y no añadir analítica ni nuevos logs de títulos. Compartir el enlace no concede acceso y una cuenta distinta no adopta automáticamente su origen.

### 2. Resolución y autorización antes del reproductor

Añadir `GET /api/v1/sources/{id}/items/{kind}/{itemId}` con `kind` interno `live|movie|series`. Reutilizar autorización de propietario y `dbCatalogItem` sobre la generación activa, devolviendo el `CatalogItem` saneado existente. IDs inexistentes y fuentes ajenas devuelven 404 indistinguible; ausencia de sesión devuelve 401. No recorrer páginas, buscar por título ni reimportar el catálogo para encontrar el ID. Errores de almacenamiento devuelven error transitorio saneado, no un falso 404.

Angular consultará `/sources` con sesión, exigirá exactamente una fuente accesible y comparará su origen compacto antes de consultar el item; cero fuentes presenta no disponible y más de una es un estado inconsistente, nunca elegir la primera. Los endpoints Go siguen recibiendo el ID real solo internamente, con autorización por propietario. Para lecturas y reproducción, conservar ese ID capturado: si la fuente cambia durante la resolución, cancelar/invalidar la operación y no reintentar sobre otra fuente. Angular resolverá item mediante el API; para un episodio resolverá primero su serie y después comprobará que su ID pertenece a los episodios de esa serie. Una fuente eliminada, sustituida o no autorizada no se sustituye por `sources[0]`; el origen del enlace debe coincidir. Sin fuente accesible se presenta un estado de contenido no disponible y regreso seguro al catálogo. Con catálogo aún sin generación activa se muestra preparación/estado y reintento, sin dar por eliminado el contenido; con generación activa, la ausencia sí es definitiva. Un fallo de red ofrece reintento en la misma URL. Cancelar/ignorar respuestas tardías de rutas, fuentes o sesiones anteriores.

Las rutas de ficha nunca abren reproducción. Entrar en una ruta de reproducción crea exactamente un playback autorizado para esa entrada, después de cerrar el anterior. Una recarga obtiene un playback nuevo; no reutiliza IDs/URLs expirados. Recuperar progreso por contentKey existente, sin alterar la política de reanudación. Mantener inicio manual si el navegador impide autoplay y los controles/diagnósticos/transcodificación actuales.

### 3. Router como fuente de selección e historial

La biblioteca conserva su presentación y puede mantener fichas modales, pero la selección visible deriva de parámetros de ruta. Usar enlaces reales para seleccionar contenido y reproducir desde la ficha, incluyendo apertura en pestaña nueva. Navegar A→B sin destruir el mismo componente también actualiza la selección. Las rutas de contenido cargan bajo demanda; no descargar todo el catálogo para resolver un item.

Cada apertura de ficha o reproductor añade una entrada significativa. Atrás/Adelante reconstruyen esa entrada y no disparan bucles ni dobles POST. Cerrar ficha/Escape vuelve al contexto interno de origen cuando exista; sin origen, vuelve al catálogo del tipo. Salir del reproductor de película vuelve a su ficha y salir de episodio a la serie/temporada; directo vuelve al catálogo. El contexto de retorno guarda filtros de catálogo y temporada en estado de navegación/servicio, acotado a usuario y fuente; no necesita exportar búsquedas privadas a la URL. Atrás del navegador conserva su semántica real, aunque el acceso haya sido desde favoritos. No se exige persistir filtros tras una recarga o compartirlos con el enlace.

Todo cambio que abandone o sustituya una reproducción ejecuta la limpieza actual (progreso VOD, cierre de playback, lease y conversión). Cambiar parámetros del mismo componente requiere la misma limpieza que `canDeactivate`: no basta con confiar en la destrucción. Proteger contra aperturas duplicadas por suscripciones y respuestas tardías; cancelar una apertura pendiente y cerrar cualquier playback recibido después de abandonarla. Restaurar foco al disparador si existe o al encabezado/contenido de destino; mantener semántica modal y teclado.

### 4. Autenticación y compatibilidad

Guardar el destino solicitado al redirigir a login y restaurarlo tras autenticación. Admitir solo rutas relativas internas reconocidas, rechazando esquemas, destinos externos, URLs `//`, redirecciones recursivas a login y formas codificadas equivalentes. El validador de returnUrl reconoce exactamente las rutas españolas y las rutas legacy admitidas, sus segmentos codificados y solo queries permitidas: en rutas españolas `o` único y `temporada` opcional solo donde corresponda; en rutas técnicas anteriores solo `season` opcional en serie/episodio; rechaza duplicados y valores fuera de gramática. No permitir `/ver` en series ni episodios en películas. Tras login se conserva el origen solicitado, sin sustituirlo por el actual. El API vuelve a autorizar la fuente para la nueva sesión; cambiar de usuario no hereda contenido, filtros ni medios. Logout/expiración limpian estado privado y cancelan playback.

`/` y `/login` siguen funcionando. Admitir las rutas anteriores `/sources/:sourceId/live/:itemId`, `/sources/:sourceId/movies/:itemId[/watch]` y `/sources/:sourceId/series/:seriesId[/episodes/:episodeId]` como entradas de compatibilidad. Autorizar primero el sourceId exacto y exigir que coincida con la fuente activa; resolver el título por ID, convertir `season` a `temporada` y reemplazar por la ruta española con origen calculado, sin abrir dos playbacks. Fuente legacy sustituida/ajena o contenido ausente: no disponible, nunca redirección al mismo ID de la fuente nueva. No generar nuevos enlaces técnicos. `/player` con selección válida en memoria se reemplaza por la ruta canónica correspondiente sin duplicar playback; sin selección recuperable muestra aviso y vuelve al catálogo, sin intentar adivinar contenido. No mantener la URL antigua como destino de nuevos enlaces. Las URLs son estables mientras existan fuente e identidad de proveedor; no redirigir por títulos similares cuando desaparece un item.

## Deployment and recovery

No cambiar Compose, variables, redes, imágenes auxiliares, PostgreSQL ni migraciones: la consulta usa tablas y claves existentes. Los jobs de migración y bootstrap existentes conservan su orden; este cambio no añade trabajo SQL. Nginx conserva el fallback SPA y la separación existente de `/api/` y assets; se confirmó estáticamente, pero no se hizo aceptación manual de rutas profundas. No operar sobre Dokploy ni producción. La API nueva conserva endpoints existentes; publicar web y API de la misma versión. Revertir imagen conserva base y biblioteca pero pierde soporte a nuevas rutas en el frontend anterior; no ejecutar down migrations.

## Risks / Trade-offs

- IDs separados y origen compacto evitan ambigüedad sin tablas nuevas; los slugs son decoración y los enlaces quedan ligados a la fuente original.
- El router exige separar resolución, selección y apertura de playback hoy mezcladas en la biblioteca; evitar una segunda lógica de reproducción paralela.
- Historial y cambios de parámetros pueden dejar leases vivos si solo se usa `canDeactivate`; comprobar ambos caminos.
- No añadir ni ejecutar pruebas automatizadas ni aceptación manual en esta sesión salvo petición del usuario; solo build/compilación y validación estructural de planificación.
