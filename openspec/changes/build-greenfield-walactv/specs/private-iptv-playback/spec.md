# Spec Delta

## Purpose

Navegar y reproducir una fuente IPTV privada con aislamiento entre usuarios, credenciales efímeras y medios autorizados sin depender de servicios anteriores.

## ADDED Requirements

### Requirement: Fuente efímera y catálogo aislado
El sistema SHALL aceptar una fuente Xtream privada por usuario, validar cuenta y cargar categorías/contenido bajo demanda para directo, películas y series/episodios. SHALL mantener secretos solo en RAM y limitar consultas de catálogo a 15 segundos y 5 MB, con caché aislada por usuario/cuenta.

#### Scenario: Alta y navegación
- **WHEN** se registra una fuente válida y se selecciona categoría o serie
- **THEN** se muestran únicamente sus títulos/episodios sin descargar todo el catálogo ni exponer URLs privadas

#### Scenario: Expiración
- **WHEN** se reinicia el API o pasan 30 minutos sin actividad válida
- **THEN** se exige reintroducir la fuente sin perder la biblioteca persistente
- **WHEN** una película mantiene heartbeat válido durante más de 30 minutos
- **THEN** no caduca por inactividad mientras siga vigente su sesión padre

### Requirement: Transporte autorizado y restringido
El sistema SHALL autorizar todos los medios por cookie, usuario y playback, reescribir HLS anidado y conservar HEAD/Range/206/416 en archivos. SHALL validar allowlist, IP pública, DNS y redirecciones antes de conectarse, sin ofrecer un proxy URL libre ni reenviar secretos de sesión.

#### Scenario: Medio ajeno o destino peligroso
- **WHEN** un usuario copia una URL ajena o un destino resuelve/redirige a red privada o no autorizada
- **THEN** no recibe medios ni se abre la conexión prohibida

#### Scenario: Reproducción y búsqueda temporal
- **WHEN** se reproduce un archivo compatible y se avanza 60 segundos
- **THEN** continúa cerca de la posición solicitada mediante rangos sin cargar el archivo entero

#### Scenario: HLS anidado
- **WHEN** aparecen variantes, KEY, MAP, segmentos relativos o BYTERANGE
- **THEN** todos se resuelven por assets opacos autorizados sin revelar credenciales proveedor

### Requirement: Lease y cancelación acotados
El sistema SHALL arbitrar por cuenta las sesiones gestionadas por WalacTV y respetar su límite. SHALL cerrar el playback anterior antes de abrir otro, liberar tras 30 segundos sin heartbeat y cancelar upstream sin retener archivos completos. Abortar un segmento SHALL no liberar el lease mientras continúe el playback válido.

#### Scenario: Conflicto y cambio
- **WHEN** dos pestañas piden una cuenta con límite uno
- **THEN** solo una abre reproducción y la otra recibe conflicto
- **WHEN** se cambia canal
- **THEN** se espera cierre confirmado del anterior antes de iniciar el nuevo

#### Scenario: Seek y fin de sesión
- **WHEN** se aborta un segmento por seek
- **THEN** se cierra solo esa lectura upstream
- **WHEN** se detiene, vence o cierra sesión
- **THEN** se revocan medios y lecturas restantes

### Requirement: Compatibilidad honesta
El sistema SHALL reproducir HLS mediante soporte MSE prioritario o fallback nativo y archivos compatibles conservando formato real. SHALL informar incompatibilidades sin cambiar artificialmente extensiones, forzar 720p ni iniciar transcodificación universal.

#### Scenario: Formato incompatible
- **WHEN** el navegador no acepta el formato o códec recibido
- **THEN** se muestra un error accionable sin prometer compatibilidad universal
