## Context

`createSource`, `loadSavedSource`, `controlCheckRedirect`, `dialPublic` y la raíz de `createPlayback` consultan `allowedHost`. Actualmente una configuración ausente se convierte en una entrada vacía mediante `strings.Split` y rechaza cualquier proveedor. El transporte de control ya mantiene el origen inicial y usa DNS público con conexión fijada a una IP validada; el transporte de medios admite destinos públicos derivados de una reproducción autorizada.

Hay cambios locales ajenos en archivos de API, rutas y documentación; la implementación debe conservarlos. Producción está vinculada a `origin/main`, `compose.dokploy.yml`, con Auto Deploy desactivado según la última comprobación del coordinador. El usuario decidió prescindir de backups para esta instalación. No se usarán credenciales ni datos reales en las pruebas de este cambio.

## Goals / Non-Goals

**Goals:** conectar proveedores públicos compatibles sin registrar dominios; conservar controles SSRF, secretos y aislamiento; mantener configuraciones restrictivas existentes y fuentes cifradas sin migración.

**Non-Goals:** aceptar toda URL o todo servicio público de Internet; ampliar puertos o redirecciones de control; introducir un proxy libre, nuevos servicios, interfaz administrativa, importación legacy o política de copias.

## Decisions

1. **Vacío significa público.** Normalizar espacios y descartar entradas vacías. Si no queda ningún host, la admisión no exige coincidencia administrativa. Si queda alguno, conservar comparación exacta de host normalizado, sin comodines ni coincidencias por sufijo. `*` no es un permiso universal. No añadir una segunda variable de modo.
2. **Una política en todos los accesos.** Reutilizar el criterio en alta, restauración, control y raíz de playback. Una fuente guardada de proveedor público seguirá restaurándose con la variable ausente. Si se activa una lista restrictiva que la excluye, conservar el registro cifrado y fallar según el contrato de errores existente; no borrar datos por un cambio de configuración.
3. **Formato y origen acotados.** Conservar `/get.php`, parámetros y límites existentes, host no vacío, HTTP o HTTPS, sin userinfo, fragmentos ni zonas IPv6. Alta/control admiten HTTP:80 y HTTPS:443 (implícitos o explícitos concordantes). El control mantiene esquema, host y puerto del origen inicial y máximo tres redirecciones; una nueva lista abierta no autoriza un salto a otro origen.
4. **Público se verifica al conectar.** La admisión por formato o lista nunca sustituye la resolución. Antes de abrir cada nueva conexión de control y medios, rechazar resolución vacía o cualquier respuesta con IP no pública, incluidas IPv4/IPv6 privadas, loopback, link-local/metadata, multicast, documentación, reservadas y variantes mapeadas a destinos privados. Rechazar DNS mixto aunque incluya una IP pública. Conectar directamente a una IP de esa respuesta validada, preservando Host y verificación TLS; no resolver de nuevo en el dial ni usar proxy ambiental. Mantener los límites existentes de puertos, tiempos, cuerpos, concurrencia y reproducción.
5. **Secretos y procedencia.** No reenviar cookies o autorización de WalacTV al proveedor. Las URLs privadas siguen cifradas en PostgreSQL y no aparecen en respuestas, errores ni logs. La raíz de medios sigue generándose desde una fuente del usuario; CDN y HLS conservan autorización por procedencia, URLs opacas, revocación y validación pública actual.
6. **Sin cambio de datos.** No hay migración ni modificación del ciclo de PostgreSQL. El rollback a una imagen anterior conserva las fuentes y claves, pero vuelve a requerir la lista de dominios; para ese rollback habría que configurarla antes de iniciar dicha imagen. Un redespliegue aplica el código y la semántica, sin resets ni importaciones como validación.

## Risks / Trade-offs

El modo público amplía los proveedores alcanzables por usuarios autenticados; la revisión debe demostrar que no permite alcanzar servicios internos ni escapar del origen de control. La lista opcional permite mantener instalaciones cerradas. “Cualquier proveedor público” no garantiza cuentas válidas, disponibilidad, soporte Xtream, puertos alternativos ni códecs reproducibles.

## Validation

Pruebas deterministas separarán admisión y dial: un cliente simulado puede demostrar el flujo de alta, pero no sustituye tests del DNS y la IP seleccionada. Cubrir configuración nil/vacía/espacios, lista restrictiva, alta en un host no inscrito, restauración y playback; rechazos de destinos privados y DNS mixto/vacío/rebinding; control de redirects mismo origen y límite, esquema/puerto inválidos, y ausencia de secretos/cabeceras. Ejecutar Go con race en paquetes afectados y pruebas de restauración con PostgreSQL exclusivamente local cuando el harness lo requiera. Validar OpenSpec estricto y Compose sin mostrar valores secretos. Las comprobaciones reales de un proveedor o producción solo las realiza el coordinador dentro de la autorización vigente.
