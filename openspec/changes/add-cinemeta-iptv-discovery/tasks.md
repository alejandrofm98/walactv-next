# Tasks

Las verificaciones de estas tareas se limitan a revisión de código y documentación, builds deterministas y validación estática de Compose/OpenSpec. No añadir ni ejecutar pruebas automatizadas, fixtures de prueba, benchmarks ni comprobaciones funcionales en ejecución salvo petición expresa del usuario. Los escenarios de la especificación conservan el contrato funcional y sirven como lista de revisión estática.

## 1. Adaptador público Cinemeta

- [x] 1.1 Implementar adaptador Go de `top` movie/series, query search/genre/skip y normalización IMDb/tipo/año; revisar código frente al contrato público documentado, etiquetas de géneros españolas y rechazo de entradas/payload inválidos; comprobar compilación Go sin ejecutar pruebas.
- [x] 1.2 Aplicar HTTPS/hosts fijos, redirects permitidos, 5 s/2 MiB, cancelación y límites globales de tasa/concurrencia/cola; revisar las rutas de redirect malicioso, cuerpo excesivo, timeout, saturación y arranque para confirmar sus límites y ausencia de llamadas externas automáticas.
- [x] 1.3 Añadir caché pública LRU/deduplicación y enfriamientos 429/5xx/transporte según diseño; revisar TTL, límites, Retry-After y tipos almacenados para confirmar que no contienen objetos IPTV o secretos.
- [x] 1.4 Documentar en `docs/cinemeta-research.md` la integración de catálogo y sus límites, manteniendo su conclusión sobre TMDB español; revisar correspondencia con el manifiesto público documentado y que la investigación histórica de IDs no se confunde con el esquema actual 004.

## 2. Intersección privada y endpoint

- [x] 2.1 Añadir consulta SQL parametrizada por fuente/generación/tipo y lote IMDb, categoría/idioma, conservando versiones; revisar predicados y orden para IMDb distinto, sin IMDb, tipo/año contradictorios, títulos traducidos y duplicados de calidad, y confirmar ausencia de llamadas TMDB.
- [x] 2.2 Implementar endpoint autenticado `/api/v1/sources/{id}/discovery`, validación previa y autorización antes de caché/red; revisar ramas 400/401/404, aislamiento entre dos usuarios, exclusión sin disponibilidad IPTV y cabecera `Cache-Control: private, no-store`.
- [x] 2.3 Implementar contextos/cursores privados, límites LRU/TTL/memoria, vinculación completa y cancelación de sesión/fuente; revisar expiración, cursor ajeno/manipulado, cambio de generación y eliminación durante una petición, incluida la reautorización antes de responder.
- [x] 2.4 Implementar paginación de hasta 40 versiones con máximo tres páginas externas, buffer, orden estable y avance idempotente; revisar transiciones para lotes con más de 40 versiones, tres páginas sin matches con continuación, fin real, duplicados, reintento de cursor y errores sin perder posición.
- [x] 2.5 Documentar contrato, disponibilidad frente a salud del stream, estados/cobertura y límites en `docs/catalog-contract.md`; revisar ejemplos contra handlers/modelos y confirmar ausencia de total externo presentado como IPTV.

## 3. Experiencia Angular

- [x] 3.1 Ampliar modelos/ApiService y estado de navegación para modo y cursores privados; revisar restauración usuario/fuente/generación y limpieza al cerrar sesión, expirar o eliminar/cambiar fuente; comprobar tipado mediante build Angular.
- [x] 3.2 Añadir entrada explícita a descubrimiento en Películas/Series y controles de género, búsqueda confirmada, categoría e idioma IPTV; revisar combinación/reset y disparadores de red para confirmar ausencia de llamadas desde Directos o catálogo habitual; comprobar build Angular.
- [x] 3.3 Añadir Anterior/Continuar, estados carga/vacío parcial/fallo/reinicio y atribución/cobertura IMDb; revisar plantilla y transiciones de estado para página dispersa, cursor caducado y fuente sin IMDb, manteniendo acceso al catálogo completo.
- [x] 3.4 Integrar apertura de fichas/rutas y conservación de foco/página al volver de playback, cancelando respuestas tardías; revisar que TMDB `es-ES` permanece bajo demanda, las versiones IPTV son independientes, una serie sin episodios no ofrece reproducción y Cinemeta no crea streams/episodios.

## 4. Integración y operación local

- [x] 4.1 Revisar el coste de intersección para el tamaño documentado de 210.381 películas y 45.620 series: confirmar consulta por lote, ausencia de N+1/lecturas completas en memoria y máximo tres consultas por tres lotes; registrar razonamiento de complejidad y límites sin benchmark ni promesa de latencia medida; si aparece un bloqueo estructural, comunicar un ajuste concreto de consulta/índice antes de ampliar esquema.
- [x] 4.2 Ejecutar compilación Go y build Angular sin pruebas y validar Compose local/productiva con `config --quiet`; revisar estáticamente arranque/salud para confirmar independencia de Cinemeta y recorridos de Directos, fuentes cifradas, catálogo habitual, favoritos y playback, sin iniciar servicios ni acceder a producción o imprimir secretos.
- [x] 4.3 Registrar evidencia de revisiones/builds y actualizar `docs/deployment.md` con la ausencia de variables/migraciones/servicios nuevos, límites de caché/cursores y rollback de imagen; revisar que migrador no añade versiones, no se modifica PostgreSQL y publicación/despliegue permanecen separados de implementación; identificar expresamente la verificación funcional y de rendimiento pendiente de solicitud del usuario.
