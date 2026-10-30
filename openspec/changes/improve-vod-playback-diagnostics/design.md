## Context

El cambio `allow-public-media-redirects` autoriza destinos públicos por procedencia del playback y conserva DNS/IP, TLS, cabeceras y aislamiento. Se mantiene esa arquitectura. El reproductor usa vídeo HTML5 directo y HLS.js; una extensión MKV/MP4 no acredita qué códecs contiene ni su compatibilidad con un navegador. Un 200/206 tampoco demuestra reproducción.

La capacidad `public-media-destinations` todavía existe solo como cambio pendiente, sin spec canónica. Este delta usa ADDED para poder validarse/archivarse en el estado actual. Antes de archivar ambos cambios, consolidar la definición nueva y el cambio anterior para que la versión canónica tenga ocho saltos y los puertos ampliados, sin sobrescritura por orden de archivo.

## Goals / Non-Goals

Identificar causas reales, ampliar puertos/saltos de medios sin proxy abierto y verificar varios títulos. No desbloquear redes privadas, aceptar cualquier puerto, omitir TLS, reenviar cookies, eludir límites del proveedor ni implementar transcodificación sin evidencia y alcance posterior.

## Decisions

1. **Errores enumerados.** Clasificar al menos estado upstream no exitoso, timeout, DNS/conexión, TLS, destino/puerto bloqueado, redirecciones excedidas y respuesta inválida. Conservar el status upstream numérico solo cuando existe. No serializar `error.Error()`, URL, hostname, Location, cuerpos upstream, credenciales ni tokens. Causas no clasificables permanecen desconocidas. Mantener compatibilidad de códigos existentes cuando sea posible y añadir campos opcionales o una consulta autorizada acotada para el último diagnóstico del playback. No descargar el vídeo de nuevo para diagnosticar.
2. **Correlación privada.** Guardar solo el último fallo en memoria asociado al playback/asset existente, sujeto a la misma propiedad, vencimiento y revocación; no crear un endpoint de URL libre ni datos públicos por ID. Si se usa consulta tras error nativo, solo se ejecuta tras fallar, sin sondeo continuo, y no puede confundir un fallo anterior con una reproducción/reintento nuevo. Logs limitados a categoría y estado; la respuesta nunca incluye errores crudos de Go.
3. **Sesión ocupada.** `409 PLAYBACK_LIMIT` produce un mensaje de sesión/cuenta ocupada en creación o heartbeat. Cerrar la reproducción al salir/liberar medios; no cerrar otras sesiones automáticamente ni multiplicar peticiones. Un límite local no prueba por sí solo una restricción del proveedor remoto.
4. **Política acotada de medios.** Admitir HTTP en 80, 8000, 8080, 8081, 8880, 8888 y HTTPS en 443, 8443. Son puertos de servicio web/vídeo acotados; otros se rechazan explícitamente. Ampliar a ocho saltos y rechazar el noveno o bucles. Aplicar idéntica validación a raíz, redirect y HLS, también en el dial efectivo. La política control/alta/importación/arte/TMDB conserva sus límites previos. No añadir variables para puertos arbitrarios en esta fase.
5. **SSRF y secretos.** Mantener IP pública en cada resolución/conexión, rechazo de DNS mixto y redes reservadas, conexión a IP validada, Host/TLS correctos y proxies de entorno desactivados. GET/HEAD, Range/If-Range y respuestas 206/416 siguen válidos; no copiar Authorization/Cookie/Referer. No relajar restricciones de red para corregir un fallo de códec.
6. **Compatibilidad verificable.** Diferenciar fallo upstream de error `MediaError`/HLS; el navegador puede indicar decodificación/formato no soportado sin identificar el códec concreto. Si el transporte entrega bytes y el navegador falla, comprobar mediante muestra acotada y herramientas disponibles el contenedor/códecs antes de proponer remux/transcodificación. No lanzar ffprobe de un URL libre ni dejar procesos o sesiones abiertos. Esta investigación la coordina el principal con una única conexión al proveedor.

## Risks / Trade-offs

Más puertos públicos y saltos amplían destinos potenciales en medios autenticados. La lista acotada, procedencia autorizada, DNS/IP y revocación reducen exposición. Los enlaces caducados, restricciones de cuenta, archivos dañados y códecs no soportados siguen pudiendo fallar. Los diagnósticos son indicios observados; no identificar como proveedor un fallo de red local sin evidencia.

## Migration Plan

Sin migración de esquema ni datos, sin variables nuevas ni cambios de servicios/volúmenes/redes de Compose. Arranque y rollback usan el migrador y bootstrap idempotentes existentes; PostgreSQL externa permanece separada. Rollback de imagen vuelve a límites de tres saltos/puertos estándar y diagnósticos genéricos sin alterar catálogo ni fuentes cifradas. Nuevos campos de diagnóstico son aditivos y clientes anteriores pueden ignorarlos. Verificar build local, Compose sin imprimir secretos y salud; ningún push ni despliegue está incluido.

## Validation

Pruebas con proveedor/DNS/dial simulados: cada causa, status upstream, ausencia de secretos, límite sesión, propiedad/vencimiento, puertos admitidos/bloqueados en raíz/redirect/HLS/dial, 8/9 saltos, DNS mixto/rebinding/redes privadas, TLS, Range/HEAD/206/416 y cancelación. Después comprobar al menos tres películas y tres episodios de al menos dos series, incluyendo MP4 y MKV si disponibles, uno por vez. Registrar título, tipo/extensión, estado, avance real y seek cuando proceda; duración breve explícita. Fallos persistentes se clasifican y documentan, sin declarar éxito global por una muestra ni ignorar formatos fallidos.
