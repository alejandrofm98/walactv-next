## ADDED Requirements

### Requirement: Proveedores públicos sin inscripción obligatoria
El sistema SHALL permitir a un usuario autenticado registrar proveedores compatibles sin inscribir sus dominios cuando `IPTV_ALLOWED_HOSTS` esté ausente, vacía o formada únicamente por entradas vacías o espacios. SHALL mantener el formato Xtream `/get.php`, parámetros, credenciales, límites y errores saneados existentes; esta política SHALL NOT convertir la aplicación en un proxy de URLs arbitrarias.

#### Scenario: Proveedor no inscrito con configuración vacía
- **WHEN** un usuario introduce una URL `/get.php` válida de un proveedor público activo y la configuración no contiene hosts efectivos
- **THEN** la API SHALL intentar verificar la cuenta mediante el transporte público validado y aceptar la fuente si el proveedor la acepta, sin exigir editar la configuración del servidor

#### Scenario: URL incompatible
- **WHEN** se introduce una URL con host vacío, ruta distinta, esquema distinto de HTTP/HTTPS, userinfo, fragmento, zona IPv6, parámetros inválidos o un puerto de control no admitido
- **THEN** la API SHALL rechazarla sin abrir una conexión prohibida ni revelar URL o credenciales en la respuesta o logs

### Requirement: Restricción opcional compatible
Una lista efectiva no vacía en `IPTV_ALLOWED_HOSTS` SHALL conservar la restricción exacta por host normalizado. Las entradas vacías SHALL ignorarse y no actuar como permiso universal; no habrá coincidencia por sufijo ni comodines. La política SHALL ser coherente en alta, restauración, transporte de control y raíz de reproducción.

#### Scenario: Instalación con lista explícita
- **WHEN** la configuración contiene un host efectivo y un usuario utiliza otro host
- **THEN** alta, control, restauración y raíz de reproducción SHALL conservar el rechazo del proveedor excluido conforme al contrato previo

#### Scenario: Fuente cifrada tras retirar la lista
- **WHEN** se restaura una fuente cifrada válida después de retirar `IPTV_ALLOWED_HOSTS`
- **THEN** la fuente SHALL restaurarse sin exigir inscripción manual y sus conexiones posteriores SHALL seguir la política de destinos públicos; no se modificarán su identidad ni datos cifrados

#### Scenario: Fuente excluida por nueva restricción
- **WHEN** una fuente guardada queda excluida por una lista explícita posterior
- **THEN** la operación SHALL fallar de forma saneada sin eliminar su persistencia ni catálogo

### Requirement: Conexiones públicas resistentes a SSRF
La ausencia de lista SHALL NOT desactivar las defensas de red. Cada nueva conexión SHALL validar esquema, puerto y todas las IP resueltas, rechazar destinos no públicos y fijar el dial a una IP validada. SHALL conservar Host, verificación TLS, proxy ambiental desactivado y límites existentes de conexión, respuestas y trabajos.

#### Scenario: Resolución pública y conexión fijada
- **WHEN** un proveedor no inscrito resuelve exclusivamente a direcciones públicas válidas
- **THEN** el dial SHALL usar una IP de esa resolución validada y preservar el nombre de host y la verificación TLS, sin depender de una segunda consulta DNS

#### Scenario: Destino privado o DNS mixto
- **WHEN** una IP literal o cualquier registro DNS es privado, local, link-local/metadata, multicast, de documentación o reservado en IPv4/IPv6, incluido un destino privado mapeado en IPv6, o la resolución está vacía
- **THEN** el sistema SHALL rechazar la conexión antes del dial aun cuando exista también un registro público o el host coincida con la lista opcional

### Requirement: Control con origen fijo y medios por procedencia
El control SHALL conservar HTTP:80, HTTPS:443, origen inicial exacto y máximo tres redirecciones. La raíz de playback SHALL derivarse de una fuente autorizada del usuario. Medios, CDN y HLS SHALL conservar política pública, procedencia, límites, URLs opacas, revocación y aislamiento existentes. No se reenviarán credenciales de sesión de WalacTV ni se copiarán credenciales del proveedor a otro origen.

#### Scenario: Redirección de control a otro origen
- **WHEN** el control devuelve una redirección que cambia esquema, host o puerto, aunque sea público
- **THEN** el cliente SHALL rechazar el salto conforme a su política anterior y no enviarle credenciales

#### Scenario: Redirección de control válida y límite
- **WHEN** el control redirige dentro del mismo origen
- **THEN** SHALL seguir como máximo tres saltos y validar nuevamente la conexión cuando sea necesaria; un cuarto salto SHALL fallar de forma saneada

#### Scenario: Reproducción de proveedor público sin lista
- **WHEN** un usuario reproduce contenido de una fuente pública aceptada sin lista administrativa
- **THEN** la raíz, CDN y HLS SHALL funcionar con la política pública y de procedencia existente, conservando rechazo de assets ajenos, cancelación y cierre de sesión
