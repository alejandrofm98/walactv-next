> Registro histórico del núcleo inicial. La persistencia de fuentes se amplía posteriormente en `docs/source-persistence-verification.md`.

# Comprobación local · 2026-10-28

## Resultado y alcance

Núcleo nuevo Go + Angular funcionando en `http://127.0.0.1:8081`, con PostgreSQL local nueva. No se conecta a la API Python, ni importa o modifica bases anteriores. No se ha publicado en GitHub ni desplegado en Dokploy.

Las pruebas deterministas usan fixtures en Go. La QA de navegador usa la lista real facilitada y autorizada por el usuario, de una reproducción en una. No se ha preparado una fixture de navegador independiente ni un ejecutor de pruebas unitarias Angular; la web se verifica mediante compilación y recorrido real de navegador.

## Comprobaciones ejecutadas

- `go test ./...`: 23 casos/subcasos aprobados, un caso PostgreSQL optativo omitido cuando no hay DSN de prueba.
- `go test -race ./...`, `go vet ./...` y `go build ./...`: correctos.
- La integración PostgreSQL optativa se ejecutó por separado contra esta base local, con loopback dentro de su namespace de red: login, cookie, CSRF, aislamiento A/B de biblioteca, favoritos, progreso y logout correctos. DSN transmitido por archivo temporal privado y eliminado; no se imprimió.
- Migración sobre base vacía, ejecución repetida y dos migradores concurrentes: correctos. Bootstrap repetido idempotente correcto. Los jobs Compose terminaron con código 0.
- `npm ci` y compilación Angular de producción: correctos. npm informó 0 vulnerabilidades en esa instalación; no equivale a una auditoría completa de imágenes.
- Compose base y override: `config --quiet` correcto. Nginx `nginx -t` correcto. API `/api/v1/health` devuelve `{"status":"ok"}`; web, API y PostgreSQL saludables.
- `.env` ignorado, permisos 600, excluido del contexto Docker. Escaneo de archivos versionables sin las credenciales reales IPTV de esta prueba. Sin remoto Git configurado.

## Navegador real

| Flujo | Evidencia observada |
|---|---|
| Login y ruta protegida | Acceso correcto; `/player` sin sesión redirige al login |
| Caducidad de sesión | Sesión local expirada de forma controlada; petición privada 401 limpia estado y vuelve al login |
| Fuente privada | Conecta por POST; campo desaparece/vacía al conectar; catálogos bajo demanda |
| Ace Stream sin IPTV | Catálogo disponible sin fuente; ID inválido rechazado sin navegación externa |
| Directo HLS | Antena 3 FHD: 1280×720, vídeo avanzando a 37 s, sin pausa; repetido a 55 s con la imagen final |
| Película MP4 | 1920×804, duración declarada 6295 s; salto a unos 771 s y avance posterior a 785 s |
| Reanudación MP4 | Al reabrir el mismo título volvió a unos 797 s, conservando categoría de películas |
| Película MKV | 1280×720, duración declarada 5929 s; salto de unos 35 a 100 s, avance posterior a 123 s |
| Episodio MKV | 1920×1080, duración declarada 5272 s; salto de unos 54 a 111 s y avance posterior a 154 s |
| Biblioteca | Favorito y progreso visibles después de reiniciar la API, incluso sin fuente conectada |
| Cambio/salida | Regreso conserva categoría; nuevas reproducciones funcionaron sucesivamente con una sola conexión |
| Logout | Vuelve al login; la integración PostgreSQL confirma denegación posterior |

Capturas sin credenciales (`walactv-next-directo.png`, `walactv-next-mp4.png`, `walactv-next-mkv.png` y `walactv-next-episodio.png`) se conservaron fuera del repositorio. Las muestras no acreditan compatibilidad de todos los códecs. No se ha probado apertura real de aplicaciones Ace Stream en Windows/Android ni dispositivos móviles físicos.

## Rendimiento observado

Build final: carga inicial 291,74 kB brutos, 82,74 kB de transferencia estimada por Angular. HLS está separado en un chunk de 586,35 kB brutos que se solicita al abrir una reproducción. Estas cifras no miden TTFF ni rebuffering. El proxy transmite archivos/rangos sin cargar el vídeo completo ni transcodificar; el proveedor y la red siguen condicionando el inicio y la continuidad.

## Hallazgos corregidos en revisión

SQL de sesión; inyección Angular tras await; parámetros Xtream; separación catálogo/episodios; lectura del límite de cuenta; metadatos más duraderos que caché; limpieza tras revocación; cierre upstream antes de cambio; caducidad del lease; navegación móvil; favoritos por clave exacta; recuperación de contexto/progreso; CDN explícitamente autorizados; manifiestos HLS sin extensión/MIME correcto; longitud HTTP 416; assets HLS acotados/deduplicados. Revisión independiente cerrada sin hallazgos pendientes comunicados.

## Pendientes de producción

Soak de dos horas, medición de memoria/goroutines/CPU/TTFF/rebuffering, carga y fallos repetidos; dispositivos y navegadores reales; ensayos más amplios de DNS rebinding/TLS y recuperación de redes; backups/restore/rollback; destino Dokploy y HTTPS; aprobación de publicación. Una réplica de API hasta disponer de coordinación compartida. En aquella versión, los CDN exigían una allowlist exacta; esta política cambia con el soporte posterior de destinos públicos dinámicos de medios.

## Metadata TMDB bajo demanda · 2026-10-29

La migración aditiva 004 se aplicó a la PostgreSQL local aislada tras generar `artifacts/backups/before-metadata.dump` con permisos privados. La app se reconstruyó y está activa en `http://127.0.0.1:8081`; API, web y PostgreSQL están healthy, y `/api/v1/health` devuelve `{"status":"ok"}`. `openspec validate add-on-demand-spanish-metadata --strict` pasa. `go test ./...` informó 54 aprobados; la prueba concurrente de admisión de temporadas pasó 20 repeticiones con `-race`. La compilación Docker de API/web completó correctamente.

La configuración local no contiene un `TMDB_API_TOKEN`, así que la metadata real informa `disabled`; el catálogo y la reproducción IPTV conservan su fallback. No se hicieron llamadas a TMDB real. La revisión independiente confirmó la separación de caché pública/decisión privada y la admisión individual de cada miss de temporada antes del vuelo compartido. Siguen pendientes las comprobaciones amplias de integración PostgreSQL y UI que figuran en OpenSpec (tareas 4.1–4.5); esta evidencia no acredita comportamiento con un token real ni publicación/despliegue.

## Rediseño inmersivo de la web · 2026-10-29

Se aplicó un sistema visual oscuro y cálido para acceso, shell, conexión/recuperación de fuente, catálogos, fichas/episodios y reproductor. Los tokens OKLCH están en `apps/web/src/styles.css`; se conservaron los contratos y handlers, y el cierre de sesión ahora se identifica explícitamente. La ficha y la confirmación de borrado enfocan una acción al abrir, contienen Tab/Shift+Tab, cierran con Escape y devuelven el foco al disparador. Si el disparador desapareció al borrar la fuente o falla la apertura de una película/episodio, el foco va al contenido principal.

`npm run build` terminó correctamente. El bundle inicial pasó de 301,28 kB / 84,76 kB estimados de transferencia antes del rediseño a 336,63 kB / 89,53 kB después (incremento de 35,35 kB / 4,77 kB); el módulo de biblioteca permanece lazy y mide 80,80 kB / 17,80 kB. No se añadieron dependencias visuales. `openspec validate redesign-immersive-web --strict` y `docker compose -f compose.yml -f compose.local.yml config --quiet` pasaron. `impeccable detect` devuelve cero hallazgos en 1280×720 y 390×844. El cambio no modifica API, Compose, variables, esquema ni migraciones.

La revisión con acceso local comprobó 320, 390 y 1440 px sin desbordamiento horizontal; el acceso se apila sin solapamiento entre 481 y 760 px y vuelve a dos columnas a 761 px. La captura final a 1280×720 mantiene formulario y pie visibles; a 390×844 el formulario ocupa una sola columna. En un harness con contenido sintético, a 320 y 390 px el error del reproductor muestra completos mensaje y botones. Con `prefers-reduced-motion: reduce`, las transiciones de carteles quedan en `0s`. En flujo sintético de biblioteca, ficha y confirmación gestionan foco, límites de Tab/Shift+Tab, Escape y retorno; al fallar la apertura de un título se restaura foco al disparador si sigue disponible. No se ejecutó el borrado ni se llamó a IPTV. Contraste calculado: texto tenue sobre canvas/superficie 6,91:1 / 6,26:1; botón principal 8,89:1.

La captura baseline y el recorrido live abarcaron la pantalla de acceso. Las capturas finales sin secretos están en `/tmp/walactv-redesign-final-1280.png` y `/tmp/walactv-redesign-final-390.png`. No había sesión autenticada ni fixture de catálogo, por lo que no se hizo inspección visual end-to-end de cada vista protegida, smoke de filtros/favoritos/Ace ni verificación de zoom 200%; esos pendientes aparecen en `openspec/changes/redesign-immersive-web/tasks.md`. Los controles y estilos de esas vistas se inspeccionaron en código y los estados puntuales del reproductor/modal se probaron con DOM sintético. Se reconstruyó solo el contenedor web; API y PostgreSQL siguieron activos y `/api/v1/health` devolvió `{"status":"ok"}`. No hay push ni despliegue Dokploy verificado.

## Identidad WalacTV frente a Nuvio/Stremio · 2026-10-29

El usuario confirmó que WalacTV no debe parecerse a Nuvio ni a Stremio. Se registró como anti-referencia en `PRODUCT.md`, `DESIGN.md` y OpenSpec. Para llevarlo a la interfaz, las secciones Directo/Películas/Series dejaron de usar el banner promocional y aro decorativo; ahora muestran un encabezado editorial compacto, contexto de la sección y un acceso claro al estado, recuento, búsqueda y filtros. La cuadrícula IPTV y sus acciones se mantienen.

La vista sintética de catálogo con datos ficticios se inspeccionó a 1280×900 y 390×844 para revisar jerarquía, cabecera, barra de estado y adaptación de controles. No se inició sesión ni se cargó una lista privada. Por tanto, esto no sustituye al recorrido visual protegido señalado en la tarea 5.2. `npm run build` pasó: bundle inicial 337,76 kB / 89,67 kB estimados de transferencia; biblioteca lazy 80,27 kB / 17,63 kB. `openspec validate redesign-immersive-web --strict` pasó. La web local en `http://127.0.0.1:8081` y `/api/v1/health` devolvieron estado healthy/`{"status":"ok"}` tras la reconstrucción. Sin cambios de API, Compose, variables ni esquema; no hay push o despliegue remoto.

## Catálogo y reproducción · 2026-10-29

En el navegador local inicié sesión y filtré Series por `PARAMOUNT | SKYSHOW [ES]`, idioma Español y búsqueda `Tierra de mafiosos`. Aparece `ES - Tierra de mafiosos (2025)` con episodios de temporada 1, incluido `S01E01 - Pegar o torcer` (MKV). Al abrirlo, el reproductor recibió HTTP 502 `UPSTREAM_ERROR` dos veces desde el proxy de medios; no llegó un flujo de vídeo. Por ello no se puede atribuir ese fallo al MKV ni al códec. Se sustituyó el mensaje incorrecto de “códec no compatible” por un diagnóstico neutral de recurso/proveedor/formato.

Tras volver desde el reproductor, la sección Series, la búsqueda, la categoría y el idioma Español siguieron seleccionados y el título volvió a aparecer. Para la limpieza de nombres, la búsqueda `###` devolvió 51 canales; los 40 de la página recibida contenían el marcador en el JSON del proveedor y ninguno lo mostró en la tarjeta. Los títulos de origen y el endpoint de reproducción se mantienen intactos. Esta comprobación no garantiza que el proveedor entregue el episodio mientras su proxy responde 502.

## Diagnóstico de reproducción · 2026-10-29

El 502 anterior al abrir `Tierra de mafiosos`, T1 E1, era consecuencia de una redirección HTTP 302 del proveedor a un servidor de medios que no estaba incluido en `IPTV_ALLOWED_HOSTS`. La petición directa a ese destino devolvió HTTP 206 con vídeo Matroska. Tras validar que el destino resuelve a direcciones públicas, se añadió su nombre exacto a la configuración privada local y se recreó únicamente el servicio API. No se registran el nombre del servidor, la URL ni credenciales.

En una nueva prueba desde la web, el episodio recibió HTTP 206 por el proxy y el vídeo avanzó durante más de cinco segundos a 1920 × 1080, sin error de reproducción. Se trata de una comprobación breve en este navegador, no de una prueba de estabilidad de la emisión completa ni de compatibilidad con todos los dispositivos. Otros destinos de medios del proveedor pueden requerir una revisión y configuración local equivalentes.

## Destinos públicos dinámicos · 2026-10-29

El proxy de medios acepta destinos públicos derivados de redirecciones y manifiestos de una reproducción autorizada, con transporte separado del control, validación de URL/DNS/IP y puerto, conexión a la IP validada, máximo tres redirecciones y eliminación de `Referer` y cabeceras privadas. La fuente IPTV sigue limitada a los dominios configurados. Las referencias HLS se reescriben como assets internos del playback.

Se retiró de la configuración local la excepción individual añadida en el diagnóstico anterior. Con la API reconstruida, `Tierra de mafiosos`, T1 E1, volvió a reproducirse mediante HTTP 206: el vídeo avanzó desde el punto guardado a 1920 × 1080, y se cerró el playback después de la prueba. El episodio usa una redirección a una IPv4 pública literal. Se comprobó salud y Compose `config --quiet`; no se acredita estabilidad de todo el episodio ni compatibilidad de todos los dispositivos. Sin cambios de esquema ni despliegue remoto.

Validación del cambio: suite Go completa y `go test -race ./internal/api` (65 pruebas), `go vet`, revisión independiente y validación OpenSpec estricta. Los fixtures cubren redirecciones HLS, DNS cambiante público/privado, TLS válido con SNI y Host, cabeceras, cierre de bodies y aislamiento entre usuarios. El caso de certificado TLS inválido no tiene fixture específico; se mantiene la verificación estándar de TLS. La reproducción real se repitió con la imagen final y se cerró después de verificar el avance.

## Diagnóstico VOD y muestra real · 2026-10-29

Se ampliaron puertos habituales de medios y redirecciones, manteniendo verificación de destino público y aislando la política de control. El diagnóstico asociado al playback informa solo categoría y estado upstream, sin URL, cuerpo ni secretos. La web distingue cuenta ocupada, errores de transporte y errores del navegador. Pasaron 77 pruebas Go con detector de carreras, `go vet`, compilación web, revisión independiente y OpenSpec. No se ha configurado ni verificado un despliegue remoto.

Tras aplicar el cambio local se probaron secuencialmente tres películas y tres episodios de dos series. Avanzaron varios segundos: `A Complete Unknown (2024)` MKV, `Absolution (2024)` versión normal MKV, `Es - Jujutsu Kaisen 0 (2021)` MP4, `Tierra de mafiosos` T1 E1 y E2 MKV y `Ella, maldita alma` T1 E1 MKV. La versión 4K MKV de `Absolution` devolvió HTTP 206 pero el navegador la rechazó con MediaError 4. Una muestra de 2 MiB analizada localmente con ffprobe identificó vídeo HEVC 3840 × 1600 y audio AC-3/E-AC-3; el error no distingue qué pista o combinación causa la incompatibilidad. Las muestras son breves y no acreditan reproducción completa, seek general ni todos los dispositivos. Los playbacks se cerraron después de cada prueba y no se registraron URLs ni credenciales.

## Transcodificación VOD acotada · 2026-10-29

FFmpeg 6.1.2 está incluido en la imagen Docker de la API. Se activa con `VOD_TRANSCODING_ENABLED=true`; en la configuración local el valor está solo en `.env` ignorado por Git. No se instala FFmpeg en el sistema anfitrión ni en el navegador. El servicio local convierte bajo demanda únicamente cuando se solicita el fallback; en producción lo ejecutaría el contenedor de API del servidor. Si el navegador ya reproduce el formato, esta conversión no interviene.

Comprobación real secuencial con `Absolution (2024)` 4K MKV: la reproducción directa falló con MediaError 4; se pidió el fallback y el backend sirvió manifiesto HLS y segmentos con HTTP 200. El vídeo avanzó durante la muestra (aprox. 4,6 s) y el navegador reportó 1282 × 534, sin duración conocida ni control de seek disponible. El intento confirma conversión y reproducción breve de este archivo, pero no una película completa. El proceso se detuvo al reiniciar únicamente la API y se comprobó que no quedaba ningún proceso FFmpeg; la API volvió a healthy.

La pausa superior a 45 s se intentó en esa misma sesión, pero la automatización del navegador quedó esperando y se reinició la API para limpiar el proceso. Por ello no se da por verificada en proveedor real la reanudación tras pausa ni el seek real. Esos flujos sí cuentan con pruebas deterministas; siguen pendientes la duración fiable para esta fuente, seek real, estabilidad prolongada y medida de CPU/memoria durante una emisión larga. Una ejecución local sintética del filtro con entrada 3840 × 1600 produjo 1280 × 534; la muestra IPTV real informó 1282 × 534, una diferencia de dos píxeles que queda anotada para revisar con otra captura de salida real. No se promete compatibilidad con cualquier códec o servidor.

Tras el cambio: `go test -race ./...` (92 pruebas), `go vet ./...`, `npm run build`, `openspec validate add-bounded-vod-transcoding --strict` y `docker compose config --quiet` pasan. API, web y PostgreSQL están healthy en `http://127.0.0.1:8081`; `/api/v1/health` devuelve `{"status":"ok"}`. No hay migraciones de base de datos ni despliegue remoto.

## Reproductores IPTV/VOD y limpieza de interfaz · 2026-10-29

La biblioteca coloca ahora los filtros en el orden búsqueda → idioma → categoría en Directo, Películas y Series. Se redujeron eslóganes, subtítulos repetidos y estados decorativos; se conservaron las etiquetas, mensajes de privacidad, compatibilidad, procedencia, idiomas aproximados y errores con reintento. Se quitó el estado redundante «Sesión privada» y la duplicación del indicador de emisión.

El reproductor distingue `live` de `movie|episode`: directo usa un escenario de pantalla más ancho, título del canal, un único indicador «En directo», controles nativos y una sola salida al catálogo. VOD conserva la composición cinematográfica, el inicio manual, los controles nativos para la reproducción directa, los controles propios de conversión, la reanudación/progreso, fullscreen y la salida que libera el lease. No se añadieron EPG, zapping, timeshift, pistas, fuentes alternativas ni navegación entre episodios. La referencia Nuvio/Stremio se limita a la composición/affordances del player VOD; la marca y biblioteca siguen siendo WalacTV.

Verificación: `npm run build` pasó (bundle inicial 344,13 kB / 90,68 kB estimados de transferencia; biblioteca y reproductor siguen lazy). `openspec validate redesign-immersive-web --strict` pasó y la revisión independiente no encontró hallazgos pendientes. Se capturó e inspeccionó el login local a 1280×720 (`/tmp/walactv-login-redesign.png`); no se inició sesión ni se cargó la biblioteca privada, así que el recorrido visual autenticado de filtros, player y fullscreen queda pendiente en las tareas 5.2, 5.4 y 5.7.

La imagen web local se reconstruyó y `http://127.0.0.1:8081/login` devolvió HTTP 200. API, web y PostgreSQL quedaron healthy; `/api/v1/health` devolvió `{"status":"ok"}`. Compose también reconstruyó la imagen API (sin cambios de código API) y lanzó los trabajos existentes de migración y bootstrap, ambos terminaron con código 0; no había migraciones nuevas. No se modificaron Compose, variables, esquema, API ni datos IPTV. No se llamó al proveedor ni se reprodujeron canales. Sin push ni despliegue Dokploy.
