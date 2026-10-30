# Design

## Context

Fuentes ya cifradas con AES-256-GCM en PostgreSQL; catálogo actual JSONB por consulta y TTL 60 s, categorías/títulos sin arte ni idioma y búsqueda limitada a selección. El endpoint de items debe consultar todo el catálogo guardado, especialmente con «Todas». Ver contrato normativo de integración `docs/catalog-contract.md` y contexto `docs/deployment.md`.

## Goals / Non-Goals

Objetivos: descubrimiento rápido sobre PostgreSQL propio, lista disponible tras reinicio, búsqueda combinable, imágenes seguras y navegación VOD útil sin perjudicar un stream activo. No objetivos: modificar proveedor, aumentar conexiones contratadas, transcodificar, importar bases anteriores, multirréplica, publicar o integrar automáticamente metadatos externos ambiguos.

## Decisions

### Índice completo y generaciones

Migración aditiva 003 crea estado/trabajo de catálogo, generaciones, categorías e items normalizados con FK a fuente y propietario. Índices prefijan fuente/generación/tipo e incorporan filtros y nombre normalizado para orden/paginación SQL. No usar extensiones PostgreSQL obligatorias. `%`/`_` del término son literales, no comodines inyectados. Consultas parametrizadas y propiedad validada antes de listar, contar, imágenes, playback o episodios.

Seis llamadas globales secuenciales: categorías e items de live/movie/series, sin category_id. Respuestas procesadas por partes, con límites de 256 MiB por respuesta y 500.000 items por tipo; petición 120 s y trabajo 10 minutos. Estos límites se ajustaron tras verificar un catálogo real de más de 210.000 películas. Tipos/IDs/extensión/category se validan y deduplican; basura no termina convertida en URL reproducible. Contadores y puntero de generación activa se publican juntos solo si los tres tipos terminan. No descargar episodios de todas las series. Conservar contenido previo hasta éxito de refresh; errores claros conservan la última generación con fecha visible, sin fingir frescura.

Un único job por fuente/cuenta y una llamada de catálogo en vuelo; POST duplicado reutiliza trabajo. Desacoplar cancelación del request POST de vida del trabajo, vinculándolo a fuente viva/proceso. Borrado/reemplazo invalida generación y cancela trabajos; guardar staging exige FK/fuente vigente. Reinicio conserva snapshot activo y convierte sync interrumpida en reintentable, no en éxito. Logout conserva catálogo autorizado por fuente y revoca reproducción según comportamiento existente; sync no concede acceso sin nueva sesión. No abrir stream/lease para sincronizar ni sostener mutex de streams mientras espera red/SQL.

### Lectura, filtros y lenguaje

GET catálogo no fuerza red. UI sincroniza una fuente sin generación y muestra progreso por estado; refresco manual CSRF permite reintentar. Mientras existe una generación se navega desde DB sin obligar una llamada al proveedor cada 60 s. Categoría ausente y lenguaje ausente abarcan todo el tipo; combinar ambos con search antes de paginar, orden nombre+ID y límites contractuales. Idiomas deducidos de etiquetas proveedor como `[ES]`, `ES|`, nombres de grupo explícitos; nombres sin evidencia se marcan `und`. No asumir que idioma de etiqueta es audio disponible. No inferir idioma por palabra incidental de un título.

### Arte privado

Extraer stream_icon/cover; persistir originales solo en ciphertext usando clave existente y AAD con propietario/fuente/tipo/ID. Frontend recibe exclusivamente `/api/v1/sources/{id}/images/{kind}/{itemId}`. Cliente separado valida DNS/IP pública en cada dial y redirección, no transmite cookies/autorización del usuario ni acepta URL libre. HTTP(S), tres redirects, 10 s, 5 MiB, firma MIME JPEG/PNG/WebP/GIF, caché acotada y private max-age. Rechazar SVG/HTML, direcciones privadas y metadata. Autorización previa a hit de caché; fallos genéricos, placeholder local y carga lazy con tamaño reservado.

### VOD y Cinemeta

Tres destinos claros Directos/Películas/Series. Películas/series usan poster cuando existe, ficha con datos proveedor disponibles y acciones reproducción/episodios existentes. No crear reparto/sinopsis/nota ficticios ni detener reproducción por falta de ficha. Cinemeta puede aportar metadatos IMDb en integración futura; el coordinador documenta investigación con fuentes oficiales, limitaciones, idioma y emparejamiento. Los títulos privados no se envían automáticamente fuera; futuras búsquedas requieren acción explícita, asociación inequívoca o selección del usuario y procedencia visible. No se añade proveedor de streams ni dependencia de Cinemeta para navegar IPTV.

## Risks / Trade-offs

Catálogos grandes pueden superar límites: conservar snapshot anterior y error `CATALOG_TOO_LARGE` con reintento, sin carga parcial oculta. Descargar catálogo global cuesta más la primera vez y evita múltiples descargas por filtro. Streaming del vídeo y sincronización usan clientes/recursos separados; un fixture demuestra ausencia de lease extra. Fallos/CDN de arte no bloquean lista. Inferencia de idioma incompleta se declara en UI. Almacenamiento PG crece por generaciones: limpiar staging fallido y generaciones antiguas tras publicación, manteniendo seguridad de referencias/requests activas.

## Migration Plan

1. Coordinador crea backup privado local de PostgreSQL antes de actualizar; conservar clave original sin imprimirla.
2. Job Compose existente ejecuta `walactv migrate` bajo advisory lock, aplica 003 una vez antes de API. No cambiar 001/002 ni la PostgreSQL externa planificada.
3. Generación inicial se llena por demanda/acción UI al restaurar fuente; un fallo no borra fuente ni obliga reintroducir secretos. Verificar reinicio, usuario distinto y concurrencia.
4. Rollback de imagen conserva nuevas tablas/columnas, versión anterior las ignora y usa su catálogo anterior. No hacer DROP/down automático ni atribuir al rollback reversión de DB. Recuperación requiere backup y clave original.

## Verification

Fixtures deterministas de varias categorías/idiomas/tipos y volumen suficiente para paginación global; fuente A/B, refresh concurrente y fallido, delete durante job, reinicio, rollback compatible y ausencia de URLs/credenciales plaintext. Proxy arte con MIME inválido, tamaño, redirects DNS privados, cache ownership. UI prueba filtros «Todas», combinaciones, respuestas fuera de orden, imagen ausente, navegación película/episodio. Coordinador verifica local navegador y proveedor real de uno en uno; no producción.
