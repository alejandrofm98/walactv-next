## 1. Contrato y gestión de trabajos

- [x] 1.1 Definir endpoints autorizados/idempotentes de iniciar/estado/seek/cancelar y respuestas saneadas; validar VOD file, offsets, generación y configuración habilitada.
- [x] 1.2 Implementar un trabajo global sin cola, compartiendo lease de playback y revocación; cancelar/esperar requests directas antes de conversión y durante seek.
- [x] 1.3 Probar límite de una conexión por cuenta, idempotencia, aislamiento, expiración/logout/eliminación fuente y transiciones concurrentes.

## 2. Gateway y proceso FFmpeg

- [x] 2.1 Crear gateway loopback exclusivo por job, sin credenciales en argv/env y con transporte público validado, Range/HEAD y conexión upstream serializada.
- [x] 2.2 Gestionar FFmpeg con argumentos fijos y demuxers/protocolos acotados; H.264/AAC hasta 720p, hilos acotados y salida HLS privada atómica.
- [x] 2.3 Consumir solo progreso numérico, límites de arranque/inactividad/vida/256 MiB y cancelación del grupo de procesos con limpieza total.
- [x] 2.4 Probar fixtures codec/contenedor, sin audio/aspecto, secretos centinela, entradas maliciosas, fallos/recursos y máximo una conexión upstream.

## 3. HLS y búsqueda temporal

- [x] 3.1 Servir manifiestos/segmentos bajo usuario/playback/generación con no-store y rechazo traversal/parciales/generaciones obsoletas.
- [x] 3.2 Reiniciar en offset para seek, con cierre anterior completo, duración fiable y progreso lógico; rechazar búsquedas inviables sin descarga ilimitada.
- [x] 3.3 Probar reproducción real de fixtures en navegador y seek/avance/progreso/cancelación con distintas generaciones, pacing para no adelantar ventana y volver/reabrir con duración desconocida sin duplicar offset.

## 4. Interfaz

- [x] 4.1 Fallback una vez para formato/decodificación observados sin fallo de transporte; acción explícita para causa desconocida y mensajes de deshabilitado/capacidad/fallo.
- [x] 4.2 Mostrar calidad compatible hasta 720p, preparación, timeline lógico cuando duración fiable y política de pausa; mantener navegación/filtros.
- [x] 4.3 Probar que 409, auth y errores de transporte no abren transcoder ni reintentos automáticos y salir destruye medios/trabajo.

## 5. Contenedor y comprobaciones

- [x] 5.1 Añadir FFmpeg/capacidades necesarias a imagen API, variable opcional y tmpfs/directorio privado; Compose limita 2 CPU, 1536 MiB y 128 procesos, sin puertos nuevos.
- [x] 5.2 Documentar habilitación local, presupuestos, compatibilidad/seek/pausa, ausencia de migraciones y rollback; mantener PostgreSQL externa y API_REPLICAS=1.
- [x] 5.3 Ejecutar Go relevante/race, web build, OpenSpec estricto, Compose config privado y salud bajo conversión; registrar versión FFmpeg y fixture reproducible.
- [ ] 5.4 Principal: validar secuencialmente MKV 4K incompatible y otro VOD, con avance sostenido/seek/CPU y limpieza; registrar límites y no prometer compatibilidad global.
