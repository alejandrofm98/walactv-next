## ADDED Requirements

### Requirement: Destinos públicos de medios autorizados por procedencia
El sistema SHALL autorizar todos los medios por cookie, usuario y playback, reescribir HLS anidado y conservar GET/HEAD/Range/206/416 en archivos. SHALL mantener allowlist para alta/control del proveedor y permitir destinos públicos dinámicos de medios únicamente derivados de redirecciones o manifiestos de una reproducción autorizada. SHALL validar esquema, puerto, método, DNS e IP pública antes de abrir conexiones, sin ofrecer proxy de URL libre ni reenviar secretos de sesión.

#### Scenario: Medio ajeno o destino peligroso
- **WHEN** un usuario copia un asset ajeno, inventa un asset o un destino resuelve/redirige a red privada o reservada
- **THEN** no recibe medios ni se abre la conexión prohibida; la respuesta y los logs no incluyen URLs ni credenciales upstream

#### Scenario: CDN público dinámico
- **WHEN** la raíz de medios generada desde una fuente permitida redirige a un host público no inscrito en `IPTV_ALLOWED_HOSTS`
- **THEN** el sistema SHALL permitir hasta tres redirecciones HTTP/HTTPS válidas y servir el medio mediante la URL opaca existente; no será necesario registrar manualmente el CDN

#### Scenario: Control conserva origen autorizado
- **WHEN** se registra una fuente con host no permitido o el control del proveedor redirige a otro origen
- **THEN** el sistema SHALL rechazar la operación conforme a su política previa aunque el destino sea público

#### Scenario: Validación de destino y DNS
- **WHEN** un medio raíz, redirect o asset necesita una nueva conexión
- **THEN** el sistema SHALL admitir solo GET/HEAD sobre HTTP:80 o HTTPS:443, sin userinfo/fragmento/zone ID, resolver DNS y rechazar hosts vacíos, respuestas vacías o que contengan cualquier IP no pública; SHALL conectar a una IP validada conservando Host y verificación TLS y SHALL impedir DNS rebinding

#### Scenario: Redirecciones limitadas y cabeceras privadas
- **WHEN** un medio presenta un cuarto salto, un bucle o un destino inválido, o la petición contiene Cookie/Authorization/Proxy-Authorization
- **THEN** el sistema SHALL cerrar la respuesta upstream al fallar, devolver error saneado y no enviar esas cabeceras al proveedor ni al CDN; SHALL conservar solo cabeceras de medios permitidas y tokens explícitos en URLs del proveedor

#### Scenario: Reproducción y búsqueda temporal
- **WHEN** se reproduce un archivo compatible servido desde un CDN dinámico y se avanza 60 segundos
- **THEN** continúa cerca de la posición solicitada mediante rangos sin cargar el archivo entero y conserva HEAD/206/416 y cancelación

#### Scenario: HLS anidado y multidominio
- **WHEN** un manifiesto redirigido contiene variantes, KEY, MAP, segmentos relativos/absolutos o BYTERANGE en otros hosts públicos
- **THEN** todos se resuelven respecto de la URL final del manifiesto por assets opacos autorizados sin revelar credenciales; cada recurso/redirección conserva validación pública y límites de manifiesto/assets, sin requerir allowlist manual ni solicitudes de precarga para validar cada URI

#### Scenario: Revocación y aislamiento de medios dinámicos
- **WHEN** vence o se cierra una reproducción, se cierra sesión o un usuario intenta consumir assets de otro usuario
- **THEN** se conservan las revocaciones y cancelaciones existentes también para los destinos dinámicos; no se crea autorización permanente para esos hosts
