## ADDED Requirements

### Requirement: Fallback VOD autorizado y acotado
El sistema SHALL ofrecer conversión HLS H.264/AAC hasta 720p para archivos movie/episode de playback autorizado cuando está habilitada. SHALL usar máximo un trabajo global sin cola y reutilizar el lease sin eludir límites de cuenta. El reproductor SHALL intentar fallback automático como máximo una vez ante fallo observado de formato/decodificación y ausencia de fallo de transporte confirmado; para causas desconocidas SHALL requerir acción explícita.

#### Scenario: MKV incompatible con transporte correcto
- **WHEN** un archivo VOD recibe 206 y el navegador informa formato/decodificación no soportada
- **THEN** se prepara un único HLS compatible, se indica calidad hasta 720p y se reproduce mediante el playback autorizado si la CPU y el archivo permiten la conversión

#### Scenario: Límite o transporte fallido
- **WHEN** hay 409, error de autenticación, transporte confirmado o transcoder ocupado/deshabilitado
- **THEN** se muestra causa accionable, sin nueva conexión concurrente, cola ni bucle de fallback

### Requirement: Entrada aislada y conexión única
El sistema SHALL cancelar y esperar la conexión directa antes de convertir, mantener máximo una request upstream por cuenta y ofrecer a FFmpeg únicamente un gateway loopback vinculado al trabajo. SHALL conservar validación DNS/IP pública, TLS, redirects, puertos y rangos. SHALL excluir credenciales, URLs upstream y secretos de argv, entorno, archivos, respuestas y logs.

#### Scenario: Transición o seek concurrentes
- **WHEN** iniciar conversión o buscar temporalmente sustituye una lectura activa
- **THEN** el trabajo anterior y su upstream se cierran antes de abrir otro; el proveedor nunca observa dos requests simultáneas de esa cuenta

#### Scenario: Contenido o destino malicioso
- **WHEN** la entrada contiene referencias arbitrarias o el gateway alcanza un destino privado/reservado
- **THEN** FFmpeg no abre protocolos/demuxers no admitidos ni URLs embebidas y el transporte bloquea la conexión prohibida

### Requirement: HLS efímero privado y seek coherente
El sistema SHALL servir segmentos completos/manifiestos por usuario, playback y generación, con no-store y sin paths libres. SHALL usar seek mediante reinicio en offset y nueva generación tras detener la anterior; SHALL guardar posición lógica y no representar ventana rodante como duración total ni simular Range arbitrario sobre bytes convertidos.

#### Scenario: Seek disponible
- **WHEN** existe duración fiable y el usuario busca una posición válida
- **THEN** se reinicia conversión en esa posición, se revocan segmentos anteriores y progreso mostrado/guardado incluye el offset de la nueva generación

#### Scenario: Duración o rangos insuficientes
- **WHEN** se desconoce duración o la fuente no permite búsqueda eficiente
- **THEN** la interfaz explica la limitación y evita un timeline completo ficticio o descargar desde cero sin presupuesto

#### Scenario: Volver y reanudar con duración desconocida
- **WHEN** el usuario vuelve al catálogo durante conversión y reabre el contenido sin duración fiable
- **THEN** se conserva la posición lógica con duración cero, los filtros permanecen y la siguiente conversión usa ese offset una sola vez, sin inventar duración ni reiniciar desde cero

#### Scenario: Usuario o generación ajenos
- **WHEN** un usuario pide segmentos de otro usuario o generación revocada, o un path parcial/arbitrario
- **THEN** no recibe archivos ni se crea una conexión upstream

### Requirement: Presupuestos y cancelación verificables
El sistema SHALL limitar codificación a dos hilos, salida por trabajo a 256 MiB, preparación a 60 s, bloqueo sin progreso a 30 s, vida a 6 h e inactividad de consumo a 60 s. Compose SHALL limitar API a 2 CPU, 1536 MiB y 128 procesos. SHALL ejecutar sin privilegios, cancelar procesos/listeners y eliminar salida tras cierre, logout, expiración, error o shutdown; SHALL matar tras gracia máxima 3 s si no termina.

#### Scenario: Archivo demasiado costoso
- **WHEN** un 4K no produce primer segmento antes del límite o se bloquea la salida
- **THEN** la conversión se cancela, recursos se liberan y se recomienda una versión compatible; no se promete reproducción continua ni se elevan recursos automáticamente

#### Scenario: Reinicio o abandono
- **WHEN** el consumidor desaparece, la sesión expira o API se reinicia
- **THEN** no quedan trabajos, conexiones ni medios persistentes reutilizables y catálogo/fuente cifrada permanecen conservados

### Requirement: Operación reproducible sin migración
La imagen API SHALL incluir FFmpeg y capacidades verificadas. La configuración SHALL usar `VOD_TRANSCODING_ENABLED=false` por defecto y activación local explícita, sin nuevos puertos públicos, servicios persistentes o cambios de PostgreSQL. SHALL mantener migrador/bootstrap y API_REPLICAS=1, y documentar límites, rollback y diferencias entre prueba breve y estabilidad completa.

#### Scenario: Imagen anterior o función deshabilitada
- **WHEN** se revierte imagen o la función no está activada
- **THEN** los medios efímeros se cancelan y la reproducción directa/catálogo siguen disponibles sin alterar datos ni ejecutar migraciones nuevas
