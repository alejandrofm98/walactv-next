## ADDED Requirements

### Requirement: Diagnóstico privado de fallos de medios
El sistema SHALL conservar un diagnóstico enumerado por reproducción autorizada que distinga status upstream, timeout, DNS/conexión, TLS, destino o puerto rechazado, exceso de redirecciones y respuesta inválida cuando exista evidencia. SHALL conservar causa desconocida cuando no pueda determinarla y SHALL excluir URLs, hosts, credenciales, tokens, cabeceras y cuerpos upstream de respuestas y logs de error.

#### Scenario: Rechazo upstream identificable
- **WHEN** el servidor de medios responde 403 o 404
- **THEN** el diagnóstico identifica respuesta upstream y su status numérico sin incluir cuerpo, URL ni Location; no lo presenta como códec incompatible

#### Scenario: Fallo de política o transporte
- **WHEN** se rechaza un puerto o se produce fallo DNS, TLS, timeout o exceso de redirects
- **THEN** se presenta la categoría observada saneada y no se transforma toda causa en una atribución genérica al proveedor

#### Scenario: Diagnóstico aislado y sin descargas duplicadas
- **WHEN** un usuario consulta el fallo de un playback ajeno, vencido o revocado, o reintenta su propia reproducción
- **THEN** no obtiene datos ajenos ni obsoletos de otro intento y la consulta no abre otra conexión al proveedor

### Requirement: Mensajes de sesión y compatibilidad con evidencia
El reproductor SHALL distinguir `409 PLAYBACK_LIMIT`, fallos de transporte disponibles y errores de formato/decodificación observados por navegador. SHALL conservar los límites de sesiones, cerrar medios al salir y evitar afirmar compatibilidad por extensión o por HTTP exitoso.

#### Scenario: Cuenta ocupada
- **WHEN** crear o mantener reproducción devuelve `409 PLAYBACK_LIMIT`
- **THEN** el usuario recibe un mensaje de sesión ocupada sin reintentos concurrentes ni cierre automático de sesiones ajenas

#### Scenario: Transporte funciona pero navegador rechaza contenido
- **WHEN** se entregan bytes correctamente y el navegador informa formato no soportado o decodificación fallida
- **THEN** el mensaje identifica el problema observado de reproducción sin atribuirlo a una restricción de puerto y la validación registra la necesidad de identificar contenedor/códecs

### Requirement: Validación real representativa y secuencial
La comprobación del cambio SHALL incluir al menos tres películas y tres episodios de dos series, con MP4 y MKV cuando estén disponibles, usando una sola reproducción activa. SHALL registrar título, tipo/extensión, resultado HTTP, avance efectivo, seek cuando proceda y límites de duración, sin guardar credenciales ni enlaces privados.

#### Scenario: Un archivo funciona y otro falla
- **WHEN** la muestra contiene éxitos y fallos
- **THEN** el informe identifica por separado los resultados y causas observadas sin declarar éxito general ni asumir que todos los fallos comparten causa
