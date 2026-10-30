# Tasks

## 1. Núcleo local API

- [x] 1.1 Crear módulo Go, esquema PostgreSQL nuevo y CLI migrate/bootstrap-admin/serve; verificar migración vacía/repetida/concurrente y bootstrap sin secretos impresos, documentando comandos reales.
- [x] 1.2 Implementar auth cookie/Origin/CSRF y biblioteca privada según docs/api-contract.md; verificar login/logout, acceso A/B, favoritos/progreso persistentes y contenido estable al reintroducir fuente con pruebas deterministas.
- [x] 1.3 Implementar fuente Xtream RAM, catálogo live/movie/series/episodios bajo demanda y límites/cache aislados; verificar cuenta inválida, 15 s/5 MB, respuesta paginada y expiración renovada por heartbeat con fixtures, documentando pérdida tras reinicio.
- [x] 1.4 Implementar proxy seguro HLS/archivos y lease por cuenta; verificar SSRF DNS/redirect/IPv6, autorización recursos, Range/HEAD/seek, conflicto max1, aborto por request y revocación con go test -race, sin timeout total corto ni buffering completo.

## 2. Web y arranque

- [x] 2.1 Crear Angular standalone oscuro/español con login/fuente/categorías/items/episodios y biblioteca; verificar build, guard, cancelación/selección, CSRF y persistencia mediante contrato compartido, sin llamadas a API anterior.
- [x] 2.2 Integrar player modular con hls.js dinámico/nativo y archivos, heartbeat/DELETE previo al cambio y progreso; verificar login sin HLS, reproducción/seek y liberación de recursos, documentando formatos no compatibles.
- [x] 2.3 Integrar AceStream ID40hex/protocolo/intent con instrucciones y catálogo de ejemplo; verificar ID inválido sin navegación, gesto explícito y mensaje iOS sin sondas localhost ni P2P servidor.
- [x] 2.4 Crear Compose base web/API y override PG local con volumen propio, Nginx same-origin en 127.0.0.1:8081 y entorno ignorado; verificar config --quiet, salud, CLI inicial y shutdown sin datos/servicios anteriores, actualizando deployment/README.

## 3. Aceptación local

- [x] 3.1 Ejecutar revisión de contrato y seguridad, pruebas Go/Angular/build y fixtures API y QA de navegador con la fuente real autorizada del flujo login→fuente→directo/VOD/episodio→seek→favorito/progreso→logout; registrar evidencia y limitaciones sin secretos, además de validación OpenSpec estricta.

## 4. Preparación productiva futura

Estos elementos quedan pendientes y no bloquean la entrega local del núcleo; impiden afirmar preparación para producción.

- [ ] 4.1 Ejecutar soak de dos horas, fallos/reintentos/cancelación repetidos y medición de memoria/goroutines/CPU/TTFF/rebuffering con presupuestos publicados; comprobar varios navegadores/dispositivos y documentar formatos reales antes de prometer estabilidad.
- [ ] 4.2 Preparar destino Dokploy/remoto/ramas/HTTPS, DB externa con backup/restore y verificación de escala single-replica; ensayar rollback compatible y revisar secretos, sin publicar ni migrar recursos reales sin autorización explícita.

## Workflow follow-up

- El usuario ya autorizó implementar el proyecto local desde cero; no se requiere nueva aprobación para iniciar apply del núcleo.
- Agenda, replays, Cast, Android, importaciones legacy, Torrentio activo, Redis/escala y adaptación FFmpeg son cambios posteriores si se solicitan. No constituyen puerta de paridad para este MVP.
- Toda publicación requiere autorización y destino real verificado; registrar por separado implementación, push y salud de despliegue.
- No marcar preparación productiva terminada por pasar pruebas locales; conservar pendientes y evidencias de lo realmente ejecutado.

## Evidence

Comprobaciones y límites registrados en `docs/local-verification.md` (2026-10-28). Se ajustó la fuente de QA: fixtures deterministas del API y navegador con la lista real autorizada; no se declara un ejecutor unitario Angular ni fixture independiente de navegador. Las pruebas prolongadas/dispositivos y publicación siguen pendientes.
