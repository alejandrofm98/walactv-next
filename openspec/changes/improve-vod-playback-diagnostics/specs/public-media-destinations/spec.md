## ADDED Requirements

### Requirement: Destinos públicos de medios autorizados por procedencia
El sistema SHALL autorizar todos los medios por cookie, usuario y playback, reescribir HLS anidado y conservar GET/HEAD/Range/206/416 en archivos. SHALL mantener allowlist para alta/control del proveedor y permitir destinos públicos dinámicos de medios únicamente derivados de redirecciones o manifiestos de una reproducción autorizada. SHALL validar esquema, puerto, método, DNS e IP pública antes de abrir conexiones, sin ofrecer proxy de URL libre ni reenviar secretos de sesión.

#### Scenario: CDN público dinámico con puertos de vídeo
- **WHEN** una raíz de medios autorizada, redirect o referencia HLS usa HTTP en 80/8000/8080/8081/8880/8888 o HTTPS en 443/8443 y resuelve exclusivamente a IP pública
- **THEN** el sistema permite el destino sin añadir cada CDN a la configuración, con un máximo de ocho redirecciones

#### Scenario: Puerto bloqueado o exceso de saltos
- **WHEN** un medio usa otro puerto, un noveno salto o un bucle
- **THEN** el sistema devuelve un diagnóstico saneado de política/redirección y cierra los cuerpos abiertos sin conectar al destino prohibido

#### Scenario: SSRF y cabeceras siguen protegidos
- **WHEN** el destino inicial, un salto o un recurso HLS tiene IP privada/reservada, DNS mixto o cambiante hacia red prohibida, userinfo, fragmento, zone ID, esquema o método inválidos
- **THEN** no se abre la conexión prohibida; se conserva conexión a IP pública validada, Host/verificación TLS y la exclusión de cabeceras privadas, incluidos Cookie, Authorization, Proxy-Authorization y Referer

#### Scenario: Control conserva su política anterior
- **WHEN** alta, importación, control, arte o metadata solicitan otro origen o puerto fuera de su política anterior
- **THEN** la ampliación exclusiva de medios no les concede nuevos destinos ni puertos

#### Scenario: Medios revocados y operaciones parciales
- **WHEN** vence o se cierra el playback, otro usuario solicita su asset, o se reproduce/busca en un archivo mediante HEAD/Range
- **THEN** se conservan revocación y aislamiento existentes y, para reproducciones válidas, HEAD/206/416, cancelación y HLS anidado funcionan bajo la nueva política
