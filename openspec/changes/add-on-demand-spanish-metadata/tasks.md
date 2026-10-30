# Tasks

Todas las tareas se ejecutan localmente, en bloques de hasta dos horas. No usar TMDB real ni IPTV real en pruebas automatizadas; no publicar. Conservar cambios ajenos y tareas pendientes de otros changes.

## 1. Contrato y persistencia

- [x] 1.1 Definir/documentar contrato aditivo autenticado de metadata de ficha/temporada: claves, estados, polling acotado o respuesta asíncrona, campos y autorización previa a caché; mantener catálogo/episodios/playback existentes. Actualizar `docs/catalog-contract.md` para la autorización de consulta al abrir ficha.
- [x] 1.2 Preparar backup privado local y migración 004 aditiva: caché canónica localizada, temporadas, decisiones privadas con huella/FK y enfriamiento global persistente. Mantener migrador/lock/transacciones y 001–003 intactos.
- [x] 1.3 Implementar repositorio PostgreSQL con TTL positivo/negativo, límite stale y reutilización tras reinicio; invalidación por huella/borrado y protección contra finalizaciones tardías.

## 2. Adaptador y resolución Go

- [x] 2.1 Añadir adaptador TMDB inyectable, Bearer backend, `es-ES`, hosts/rutas fijos, validación y normalización de campos/arte, límites de tiempo/tamaño; no registrar token ni respuestas raw.
- [x] 2.2 Implementar extracción de ID estructurado cuando exista, normalización conservadora de título/año y resolución única verificable; ausencia/ambigüedad sin asociación, sin interpretar IDs IPTV como TMDB.
- [x] 2.3 Implementar consultas de ficha/temporada bajo demanda y combinación por temporada/episodio, singleflight por clave, tasa/concurrencia/cola acotadas y negativas semánticas separadas de errores.
- [x] 2.4 Implementar stale inmediato y refresco, `Retry-After` persistente, enfriamientos y estados seguros de token ausente/inválido, saturación y fallo; impedir consultas desde listados/sync/playback.

## 3. Web y configuración

- [x] 3.1 Añadir modelos/servicio Angular y enriquecimiento independiente al abrir ficha; datos IPTV/control playback inmediatos, cancelación/respuestas antiguas y estados discretos de metadata.
- [x] 3.2 Enriquecer solo temporada seleccionada y episodios IPTV coincidentes; procedencia, idioma honesto, fallback de campos/arte y atribución/logo TMDB según condiciones.
- [x] 3.3 Añadir variable opcional `TMDB_API_TOKEN` solo al runtime API en Compose y ejemplo vacío; documentar uso secreto local, límites/TTL, deshabilitación, migración/rollback y PostgreSQL externa en deployment. Conservar setup/secretos actuales y `API_REPLICAS=1`.

## 4. Comprobaciones deterministas y revisión

- [ ] 4.1 Con proveedor HTTP mock, reloj controlable y fixtures probar IDs/título/año/remakes/ambigüedad/sin año, normalización, campos españoles vacíos, payload inválido, arte host fijo y correspondencia de episodios/especiales. Cero acceso a servicios reales.
- [ ] 4.2 Probar caché PostgreSQL tras reinicio, TTL/negativos/stale máximo, misma obra en dos calidades, cambio de huella, borrado concurrente y autorización A/B antes de caché/red; comprobar ausencia de datos privados en caché canónica.
- [ ] 4.3 Probar singleflight concurrente, límites de usuario/tasa/cola, timeout/tamaño, 429 con Retry-After segundos/fecha/inválido y persistencia tras reinicio, 401/403/5xx; asegurar que errores no borran dato previo ni se guardan como sin coincidencia.
- [ ] 4.4 Probar UI con mock: ficha inmediata, playback durante loading/error/disabled, selección de temporada sin precarga, cancelación/respuesta antigua, atribución y fallback. Afirmar cero llamadas TMDB en importación/listados/reproducción.
- [ ] 4.5 Ejecutar checks Go/web existentes y migración sobre PostgreSQL local aislada con datos 003, dos migradores concurrentes/repetidos y compatibilidad de imagen anterior. Validar Compose con `config --quiet`, health sin token y ausencia del secreto en bundles/respuestas/logs sin volcar `.env` ni configuración resuelta.
- [x] 4.6 Actualizar evidencia local, validar OpenSpec estricto y obtener revisión independiente del coordinador; registrar limitaciones de identidad/primera consulta fría y separar implementación de publicación/despliegue pendientes.
