# Design

## Context

Proyecto nuevo autorizado el 2026-10-28 en la raíz de este repositorio; en esa fecha aún no tenía remoto ni servicio Dokploy. Ver `proposal.md` y contrato vinculante `docs/api-contract.md`. La decisión posterior del usuario permite implementar ahora; las puertas de paridad/importación del plan anterior no se trasladan a este MVP.

La investigación previa verificó reproducción corta y seek +60 s en MP4 real y muestras MKV H.264/AAC; no acredita todos los códecs ni estabilidad de dos horas. El proveedor observado admite una conexión y HTTP. Ningún secreto real forma parte de este diseño. Los servicios anteriores permanecen ajenos a este repositorio.

## Goals / Non-Goals

**Goals:** vertical local completo con Go API, UI nueva Angular, fuente privada por usuario, directo/VOD/episodios, biblioteca, apertura AceStream y arranque reproducible con DB nueva.

**Non-Goals:** publicar, importar usuarios, mantener API Android, paridad de agenda/replays/Cast, Torrentio activo, motor P2P servidor, HA o transcodificación universal. No condicionan el cierre local; se planifican después si se solicitan.

## Decisions

### Estructura y contrato

`apps/api`: monolito modular Go con net/http, PostgreSQL/pgx, identidad, catálogo y playback. `apps/web`: Angular 20 standalone, tema oscuro, español, features lazy y hls.js versionado importado al reproducir. Las herramientas verificadas permiten este arranque; fijar versiones/lockfiles reales al implementar. No copiar el player anterior ni llamar a la API Python. Concurrencia normal de Go y GOMAXPROCS disponible; un proceso no significa un único núcleo.

`docs/api-contract.md` fija métodos, bodies y respuestas sin envelope; todos los workers lo leen. `/api/v1` del mismo origen vía Nginx. Error estable `{error:{code,message}}`, nunca upstream bruto. Salud `/api/v1/health` y alias `/healthz`. Contrato de progreso en **segundos**, no mezclar con ms de sistemas anteriores. Endpoints y nombres de campos se cambian solo coordinadamente con cliente/API.

### Identidad y biblioteca

Bootstrap administrativo por CLI `walactv bootstrap-admin` desde variables locales ignoradas, sin argumentos/logs con contraseña. Sin registro público. `walactv migrate` aplica SQL antes de `walactv serve`. Guardar hash resistente y sesiones opacas aleatorias en DB; cookie `walactv_session` HttpOnly/SameSiteStrict, Secure salvo HTTP local. Mutaciones protegidas exigen X-CSRF-Token y Origin exacto APP_ORIGIN; login valida origen. Cookies/CSRF nunca se reenvían a proveedor.

TTL app 1800 s inactivo/43200 s absoluto; heartbeat válido renueva inactividad, nunca vence el absoluto ni resucita revocación. Logout revoca fuente, leases y medios y después elimina cookie. Biblioteca PostgreSQL por usuario+contentKey estable. `PROVIDER_IDENTITY_KEY` persistente calcula HMAC sobre host/raíz normalizados e identificador canónico de cuenta validada **sin contraseña**. accountKey sobrevive reinicio/reintroducción/rotación de clave del proveedor; ID fuente sigue siendo aleatorio efímero. contentKey incorpora accountKey+tipo+ID y es validado por servidor. La clave HMAC no se regenera al iniciar. Favoritos/progreso no almacenan credenciales.

### Fuentes, catálogo y leases

Una fuente activa por usuario, URL get.php por POST; secretos RAM y borrado al reiniciar/logout/eliminar o 1800 s inactivo. Heartbeat/actividad válida renuevan fuente mientras sesión app siga válida, para películas largas. Cuenta debe estar activa; obtener categorías live/movie/series, contenido por categoría y episodios por serie mediante adaptador Xtream. Extensión/URL final proceden de catálogo verificado en servidor, no de input del cliente. Timeout control 15 s, máximo catálogo 5 MB; caché 60 s y coalescing por usuario+cuenta+operación+parámetros. Paginar/buscar después de la respuesta por categoría si upstream no admite paginación.

Lease concurrency-safe por identidad de cuenta global sin usuario, respetando max_connections declarado. Segunda pestaña recibe 409; UI espera DELETE confirmado antes de POST siguiente. Heartbeat 10 s, liberar a los 30 s sin señal. Estos límites controlan conexiones WalacTV; otros clientes pueden agotar el proveedor. Store privado RAM exige un proceso/réplica; arranque rechaza escala declarada mayor que uno y docs exigen verificar topología real. No HA ni Redis inicial; coordinación compartida y fail-closed antes de escalar.

### Transporte seguro

Cada medio exige cookie app válida, pertenencia y lease, también en HLS nativo. Reescribir manifiestos/variantes/KEY/MAP/segmentos/BYTERANGE a assets opacos bajo `/media/{playback}/{asset}`. No URLs con secretos en browser, informes o logs. HLS requiere parser/rewrite de atributos URI y rutas relativas, no reemplazo textual global. Evitar ciclos/profundidad excesiva.

Allowlist administrativa IPTV_ALLOWED_HOSTS y validación de esquema/puerto/IP pública, DNS y cada redirect/URI HLS. Resolver+validar+fijar IP al dial conservando Host/TLS ServerName y verificación de certificado; bloquear rangos privados IPv4/IPv6/metadatos y DNS rebinding. No proxy URL libre. Upstreams fixture locales requieren opción de test explícita y nunca configuración productiva. Error/log sanitizados; no reenviar Authorization/cookies/hop-by-hop.

Transport Go reutilizable con timeout de conexión/TLS/headers y streaming por chunks con backpressure. Sin timeout global Client.Timeout/Server.WriteTimeout corto que corte VOD; separar catálogo de medios y usar límites renovables de inactividad. Cancelar contexto cierra solo la request/body upstream; buscar o ABR no destruye lease. DELETE/logout/expiry/heartbeat timeout cancela todas las requests de playback. HTTP HEAD/Range/206/416 y Content-Range conservados; nunca cargar archivo entero.

HLS por `Hls.isSupported()` primero, native como fallback. MP4 y MKV dependen del códec real; no cambiar extensiones ni forzar 720p. No incluir FFmpeg en este núcleo: incompatible produce mensaje y posible adaptación se decide después. UI libera HLS, vídeo y lease al salir/cambiar; muestra error/conflicto y permite reintentar.

### Interfaz y apertura externa

Pantalla login, fuente con campo secreto que se limpia tras envío, categorías/títulos/episodios y reproductor modular, favoritos/progreso. Cancelar solicitudes/ignorar respuesta antigua al cambiar filtro. Navegación conserva selección y estado; etiquetas/teclado/foco y carga/error explícitos.

AceStream: entrada ID40hex o acestream:// normalizado y catálogo de ejemplo inmutable; Windows/Linux protocolo y Android intent desde mismo gesto. No HTTP probes localhost ni motor backend; iOS muestra instrucciones de incompatibilidad AceStream, IPTV permanece usable. Torrentio solo se documenta como futuro/desactivado, sin afirmar implementación o enlaces probados.

### Compose y migraciones

`compose.yml`: web Nginx y API Go con DATABASE_URL externa y sin DB productiva en ciclo Compose. `compose.local.yml`: PostgreSQL nuevo con volumen/proyecto propios y variables locales generadas en .env ignorado. Web `127.0.0.1:8081` evita el prototipo 8080. API no expuesto a host, proxy fijo Nginx a red interna. Gzip, cache immutable de assets con hash, index revalidable y SPA fallback. Media proxy_buffering off/timeouts acordes con streaming largo.

Migración SQL numerada: comando Go explícito con advisory lock sobre conexión dedicada y tabla de versiones. Job one-shot antes de API; no ejecutar desde todas las réplicas. DB nueva contiene usuarios, sesiones, favoritos, progreso; no importar ni escribir en otros esquemas. Fallo aborta arranque/readiness. Cambios aditivos conservan imagen anterior; rollback de imagen no ejecuta down automático.

Variables: DATABASE_URL, APP_ORIGIN, SESSION_SECRET, PROVIDER_IDENTITY_KEY, IPTV_ALLOWED_HOSTS, BOOTSTRAP_ADMIN_USERNAME/PASSWORD; solo referencias, nunca valores. SESSION_SECRET/IDENTITY_KEY estables en .env ignorado; no usar secretos proveedor como build args. Validar Compose con config --quiet sin imprimir config resuelta. Producción, recurso PostgreSQL/red/SSL/backups, dominio, remoto/ramas/Auto Deploy siguen sin definir y requieren verificación/autorización futura.

## Risks / Trade-offs

- [RAM y reinicio] → se pierde fuente y se pide reentrada; biblioteca/sesiones app son persistentes, sin promesa de reanudación transparente.
- [Proveedor HTTP/códecs] → proxy resuelve mixed content del navegador, no cifra upstream ni vuelve universal su formato.
- [Una instancia] → límites documentados y rechazo de escalado no coordinado; no prometer alta disponibilidad.
- [Primer corte local] → contratos y fixtures verifican el núcleo; prueba real breve no acredita reproducción completa ni todos los dispositivos.

## Migration Plan

1. Ejecutar migración y bootstrap solo contra DB local nueva; comprobar esquema/versión y salud.
2. Arrancar web/API local y verificar contrato con fixture, navegador y flujo real coordinado de una conexión si se dispone de autorización y fuente.
3. Entregar evidencia del núcleo y pendientes explícitos; no retirar servicios previos ni esperar paridad legacy.
4. Futuro productivo separado: soak de dos horas, dispositivos reales, carga y escalado, backup/restore, configuración HTTPS/Dokploy y revisión de secretos. Tras autorización nueva se podrá publicar; un push no demuestra despliegue. No activar producción antes de esas comprobaciones.
