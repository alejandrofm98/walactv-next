# Design

## Context

Ver motivación en `proposal.md`. Evidencia local del 2026-11-01: `player.component.ts` usa hls.js 1.7.3 para HLS y `video.src` para archivos; no enumera pistas. Los tipos instalados de hls.js exponen audioTracks/audioTrack, subtitleTracks/subtitleTrack/subtitleDisplay y eventos. La API no entrega streams del archivo. `playback.go:rewriteManifest` opacifica atributos URI, incluidos EXT-X-MEDIA, sin conocer codecs. `vod_transcode.go` ya tiene gateway 127.0.0.1, entrada serializada, slot global, transición que drena reproducción directa, generación/offset y cancelación. Su FFmpeg actual mapea primer vídeo/audio y descarta subtítulos. `docker/api.Dockerfile` instala el paquete ffmpeg, pero solo verifica libx264/AAC.

No hay specs principales archivadas. Los cambios `redesign-immersive-web`, `add-bounded-vod-transcoding`, `build-greenfield-walactv` e `improve-vod-playback-diagnostics` siguen siendo contratos relacionados; este cambio amplía las capacidades de pistas, manteniendo sus límites y aislamiento de Live.

Fuentes primarias consultadas el 2026-11-01: [API Hls](https://hlsjs.video-dev.org/api-docs/hls.js.hls), [audioTracks nativo](https://developer.mozilla.org/en-US/docs/Web/API/HTMLMediaElement/audioTracks), [textTracks](https://developer.mozilla.org/en-US/docs/Web/API/HTMLMediaElement/textTracks), [ffprobe](https://www.ffmpeg.org/ffprobe.html), [FFmpeg y salidas múltiples](https://ffmpeg.org/ffmpeg.html) y [formatos de subtítulos](https://www.ffmpeg.org/general.html). La disponibilidad nativa de audio es limitada; soporte de codec no garantiza soporte de todo contenedor. Los tipos instalados y pruebas de la imagen decidirán la disponibilidad real, sin adoptar APIs de versiones más recientes por accidente.

## Goals / Non-Goals

**Goals:** controles inmersivos VOD, selección HLS/nativa cuando esté disponible y acceso a pistas embebidas soportadas de MP4/MKV mediante inspección y conversión acotadas. No basta con ocultar todas las pistas de archivos por falta de API del navegador.

**Non-Goals:** modificar Live, OCR/quemado de PGS/VobSub, subtítulos externos de terceros, preservar estilos avanzados ASS, todas las combinaciones de contenedor/codec, persistencia de preferencias, nuevos recursos o despliegue.

## Decisions

### 1. Presentación VOD separada del motor y de Live

Aplicar solo a movie/episode: vídeo dominante, título/salida única y banda inferior discreta con controles propios play/pause, volumen/silencio, timeline y fullscreen. Usar tokens oscuros/latón y marca WalacTV. Mantener controles nativos cuando plataforma/fullscreen solo de vídeo lo requiera; no duplicar barras. El fullscreen de contenedor incluye menús y cues; el nativo puede limitar selección a controles del sistema, disponibles los propios de nuevo al salir.

Ocultar controles tras tres segundos de reproducción sin interacción, nunca con foco dentro, menú, pausa, carga o error; restaurar por movimiento, toque o teclado. Reservar lectura de cues, safe areas, 44px, 200% zoom y movimiento reducido. Información de pistas ofrece límites concisos y acción explícita `Buscar pistas del archivo`; no sondear automáticamente todos los títulos. Elegir pista que requiera conversión explica `Preparar versión compatible · hasta 720p` antes de iniciar. Solo se muestra selector habilitado cuando hay elección soportada: audio con dos o más pistas, subtítulos con una o más y Desactivados. Inventario no disponible, parcial o pistas de imagen se explican aparte.

Alternativa de solo controles nativos: insuficiente para composición y pistas convertidas; se mantiene como fallback de plataforma.

### 2. Adaptadores de pistas y fuente de verdad por intento

Estado por playback/intento/grupo: inventario, selección efectiva, cambio pendiente y razón de indisponibilidad. HLS usa listas seleccionables del grupo vigente y eventos de actualización/cambio de hls.js 1.7.3; no usar inventario allTracks de otros grupos como opciones directamente. Desactivación de subtítulos mediante mecanismo soportado de índice/visualización. Para archivo o HLS nativo, detectar audioTracks controlables y textTracks subtitles/captions, con eventos. No mostrar dos selectores sobre los mismos cues de hls.js.

Etiquetas de language/name/label reales, nombres neutrales numerados si faltan y desambiguación de duplicados; nunca inferir idioma del catálogo. Confirmar cambio con estado real; invalidar índices al cambiar grupo y limpiar listeners/cues al sustituir intento. No persistir entre títulos. Si native controla pistas, preferirlo sin conversión; si no, permitir inventario servidor y conversión habilitada. API ausente no demuestra ausencia de pistas.

### 3. Inspección explícita, privada y acotada

Añadir `POST /api/v1/playbacks/{id}/tracks` para archivos propios y conversión habilitada; HLS mantiene inventario del motor. Respuesta aditiva: estado complete/partial/unavailable, revisión efímera y pistas con ID opaco, tipo, idioma/label saneados, codec permitido y razón de no seleccionable. No retornar tags arbitrarios, rutas o URLs. Cachear solo durante ese playback con identidad del asset; invalidar si cambia recurso/ETag/Last-Modified observado, fuente o sesión. Una revisión es evidencia limitada, no garantía de decodificación; el fallo real se trata explícitamente.

Reutilizar slot global sin cola y transición de cierre/drenaje de lecturas antes de ffprobe por gateway loopback. Validar propietario, feature y disponibilidad antes de interrumpir vídeo. Guardar posición y estado previo, pausar/drain, inspeccionar y restaurar directo/reiniciar generación anterior desde posición lógica si corresponde. Inspección sin selección no deja el vídeo detenido. Si ya hay inventario válido devolverlo sin cortar reproducción. Solicitud cancelada o logout cancela probe.

Limitar ffprobe a 10 s, 16 MiB transferidos totales en gateway, 32 requests seriales, stdout JSON 256 KiB, máximo 64 streams y labels 128 caracteres. Aplicar `probesize`/`analyzeduration` y show_entries mínimo; los límites de bytes/tiempo se imponen además en Go, pues flags solos no acotan todos los seeks. Usar demuxer permitido por extensión, listas de protocolos/demuxers existentes, sin URL privada en argv/env/logs y stderr descartado/saneado. No incluir hash/packets/frames ni descargar archivo completo. Presupuesto excedido da estado limitado, sin declarar cero pistas. Sin Range eficiente puede no descubrir metadata MP4 al final dentro del límite: informar y continuar.

Alternativas: ffprobe directo a URL privada, rechazado por secretos/SSRF; probe al abrir cada vídeo, rechazado por latencia/conexiones; extracción completa, rechazada por presupuesto.

### 4. Selección de audio con reinicio acotado

Extender POST transcode de forma aditiva con revisión e IDs opcionales de audio/subtítulo. Omitirlos conserva primer audio/default y sin subtítulos para clientes anteriores. IDs se resuelven servidor contra el inventario de ese playback; nunca aceptar expresiones -map ni índices arbitrarios. Validar tipo, codec/capacidad de imagen y revisión antes de parar trabajo. La idempotencia actual debe comparar offset y selección; mismo offset con otro audio exige generación nueva.

Tomar posición lógica actual y estado playing/paused, detener y esperar proceso/upstream previo, crear generación con mismo lease, mapear vídeo y audio elegidos y restaurar estado. Mantener elección en seek y pausa larga del mismo playback, revalidándola si inventario cambia; no perderla por defaults al regenerar. Añadir IDs efectivos a la vista de conversión sin romper campos previos. Audio no decodificable o fallo de generación presenta causa accionable y permite volver a default/reintentar; sin check de éxito falso ni loop automático.

### 5. Subtítulos textuales en la misma entrada FFmpeg

Permitir SubRip, ASS/SSA, mov_text y WebVTT solo donde el demuxer/decoder de la imagen los admita. Convertir a texto WebVTT, sin estilos/adjuntos ASS. PGS/VobSub y desconocidos se inventarían con motivo no soportado, sin OCR ni controles de activación. Mapear explícitamente solo pista elegida; un archivo sin audio también sigue siendo válido.

Elegir como salida secundaria del MISMO proceso FFmpeg un WebVTT a pipe privado, junto al HLS A/V existente. Go consume únicamente cues completos, valida tiempos/texto y conserva una ventana acotada en memoria; no extrae toda la película ni abre segunda entrada. Usar flush de salida y demostrar entrega incremental con fixture antes de dar tarea por terminada. Un pipe de cues separado no sustituye canal de progreso; gestión de cierre/wait evita deadlock y espera lectores antes de liberar generación.

Endpoint `GET /api/v1/playbacks/{id}/transcode/{generation}/subtitles.vtt` con propietario, generación, no-store y MIME text/vtt devuelve snapshot de ventana. Máximo 512 cues, 2 MiB y 16 KiB por cue, ventana alineada al HLS rodante (48 s) más cues aún activos; exceder presupuesto produce error saneado sin crecimiento ilimitado. Normalizar timestamps a posición lógica absoluta en servidor; cliente los transforma al reloj media de generación usando offset confirmado y elimina cues antiguos. Poll 2 s solo durante playback con subtítulos seleccionados; detener al ocultar/desactivar/salir y refrescar al reanudar. Crear VTTCue de texto seguro con parser WebVTT acotado, sin innerHTML, sin ejecutar tags ni abrir URLs de cues. No persistir archivos/cues completos.

Cambiar pista textual reinicia generación desde posición actual; Desactivados oculta cues inmediatamente y puede parar salida textual en la siguiente regeneración sin retrasar la acción visible. Seek y regeneración vacían cues, cancelan fetches viejos y reajustan reloj; pruebas con cues no nulos tras seek y pausas verifican sincronía. El audio continúa aunque un fallo exclusivo de parseo/cues permita desactivar subtítulos y avisar; fallo fatal de FFmpeg conserva recuperación VOD existente.

Alternativa HLS con rendición WebVTT segmentada: válida, pero introduce manifiesto maestro y coordinación/retención de segmentos extra sobre salida actual media-only; se prefiere ventana VTT privada por ser acotada y servir nativo y hls.js igual. Alternativa segundo FFmpeg extractor: rechazada por segunda conexión o lectura completa. Si la imagen no entrega WebVTT incremental aun con flush, es un bloqueo concreto para el coordinador: solución alternativa acotada es salida segmentada WebVTT del mismo proceso, conservando misma API de ventana y límites; no sustituir por extracción completa silenciosa.

### 6. Contratos operativos conservados

Inspección, conversión A/V y cues comparten slot global y presupuestos API existentes: 2 CPU, 1536 MiB, 128 procesos y 256 MiB efímeros totales; la ventana de texto cuenta dentro del presupuesto de memoria/salida, no suma un volumen. Se mantienen 60 s preparación, 30 s stall de A/V, 6 h vida y 60 s idle; ausencia de cues durante escenas sin diálogo no es stall. Probe no coexiste con conversión activa: guardar selección y detener/reiniciar si no hay caché. Kill máximo 3 s y limpieza de listener/pipe/caché al cerrar, logout, eliminación de fuente o shutdown.

Las rendiciones HLS originales siguen usando assets opacos y transporte existente, con pruebas de EXT-X-MEDIA y autorización. No elevar límite de leases ni relajar SSRF. Las reglas de entrada única para probe/conversión se comprueban en proveedor simulado; no confundirlas con las solicitudes multirendición HLS ya admitidas.

## Risks / Trade-offs

- [Inspección interrumpe brevemente] → Solo petición explícita, caché y restauración de estado/posición.
- [Inventario parcial o recurso cambiante] → Revisión efímera, límites duros y rechazo de IDs desactualizados; no afirmar ausencia por error.
- [CPU no convierte 4K en tiempo real] → Mantener límite hasta 720p y presupuestos; selección no promete estabilidad universal.
- [ASS pierde formato o bitmap no convertible] → Información visible del formato y fallback; texto limpio sin adjuntos/OCR.
- [WebVTT incremental o timestamps varían por contenedor] → Fixture por formato, offset no cero, pausas/seek y prueba de la imagen; alternativa segmentada del mismo proceso ante bloqueo demostrado.
- [Cues demasiado largos/densos o maliciosos] → Parser y ventanas con límites, texto seguro, error recuperable sin afectar memoria indefinidamente.
- [Fullscreen nativo limita menús] → Mantener controles nativos y acceso a opciones propias al salir; registrar límites de plataforma.

## Migration Plan

Actualizar Go, frontend y comprobaciones de imagen: verificar `ffprobe`, encoder/muxer WebVTT y decoders textuales del paquete FFmpeg ya instalado. Sin servicio, puerto, volumen, Compose ni variable nuevos; `VOD_TRANSCODING_ENABLED=false` conserva default y también deshabilita inspección/conversión de pistas. No activarla en producción como efecto de este cambio. API_REPLICAS=1 y tmpfs existentes se mantienen. PostgreSQL externa a Compose, sin cambios de esquema/migraciones; el migrador/bootstrap existente mantiene su orden sin trabajo nuevo.

Compatibilidad: campos nuevos opcionales de transcode preservan clientes anteriores; frontend contra API anterior (404/opciones no soportadas) informa indisponibilidad sin impedir reproducción default. Un rollback de API/web cancela generaciones y borra inventarios/cues temporales, sin cambios de datos. No se publica sin autorización/destino verificado por coordinador.

## Validation

Fixtures generados MP4 H.264/AAC con dos audios distinguibles y mov_text; MKV con dos audios y SRT/ASS; sin audio; bitmap no soportado; metadata tardía, stream malicioso y probe agotado. Probar hls.js multiaudio/WebVTT original y rutas nativas con/sin APIs. Confirmar audio audible y cues visibles sincronizados antes/después de seek no cero, pausa larga, cambio de selección, retry y salida, sin nueva conexión simultánea ni lease. Go tests con race para probe/reinicio/cancelación, revisión/ID inválido, autorización, límites y ausencia de secretos en argv/env/logs. Build imagen verifica capacidades y arranque/flush incremental; build web y recorrido responsive/teclado/fullscreen/Live completan aceptación. Solo proveedores simulados; muestras privadas reales las coordina principal.
