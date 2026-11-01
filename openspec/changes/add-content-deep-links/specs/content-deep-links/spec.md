## ADDED Requirements

### Requirement: URLs canónicas de contenido privado
La web SHALL usar rutas españolas con slug de título seguido por ID decimal, sin ID largo de fuente en el path. SHALL resolver la fuente única desde sesión y `/api/v1/sources`, exigir referencia compacta `o` y usar enlaces navegables sin credenciales, URLs IPTV, tokens de sesión/medios ni IDs temporales de playback.

#### Scenario: Selecciones y nueva pestaña
- **WHEN** se eligen dos items distintos, se abre una ficha o se reproduce un episodio
- **THEN** la URL española contiene IDs y títulos legibles, distingue ficha de reproducción y permite cargar el destino en otra pestaña autenticada

#### Scenario: Forma exacta de las rutas
- **WHEN** se genera un enlace de canal, película, reproducción de película, serie o episodio
- **THEN** se usa respectivamente `/directo/:slug-:itemId?o=:origen`, `/peliculas/:slug-:itemId?o=:origen`, `/peliculas/:slug-:itemId/ver?o=:origen`, `/series/:slug-:seriesId?o=:origen` o `/series/:slug-:seriesId/episodios/:episodeLabel-:episodeId?o=:origen`; episodeLabel es `t{season}e{episode}`

### Requirement: Slugs decorativos y normalización inequívoca
La web SHALL usar IDs y origen como identidad y slugs decorativos. SHALL extraer el sufijo ID de 1–32 dígitos desde el último guion como string. SHALL generar slugs de 1–80 caracteres del título IPTV saneado con NFKD sin diacríticos, minúsculas ASCII y guiones, con fallback por tipo. SHALL rechazar slugs inválidos y reemplazar los válidos desactualizados antes de reproducir, sin añadir historial ni recrear playback.

#### Scenario: Título modificado o slug distinto
- **WHEN** el origen e IDs son válidos y el slug válido no coincide con el título actual
- **THEN** se reemplaza la URL por la canónica del mismo contenido, sin búsqueda por título ni selección alternativa

#### Scenario: Slugs iguales o malformados
- **WHEN** dos títulos comparten slug o un enlace tiene slug vacío, demasiado largo, controles o separadores codificados
- **THEN** los IDs distinguen los títulos iguales; el enlace malformado no resuelve ni reproduce contenido

### Requirement: Origen compacto y sustitución de fuente
La web SHALL exigir `o`, exactamente base64url sin padding de los primeros 12 bytes de SHA-256 del ID de fuente en UTF-8, y compararlo con la única fuente activa de la sesión antes de resolver contenido. SHALL mantener autorización Go por propietario e ID real de fuente. SHALL invalidar resoluciones pendientes al cambiar fuente/sesión y nunca adoptar otra fuente por coincidencia de título o ID.

#### Scenario: Fuente sustituida con IDs reciclados
- **WHEN** se elimina o sustituye la fuente y la nueva contiene el mismo ID y título
- **THEN** el origen diferente hace que el enlace anterior sea no disponible, sin consultar/reproducir el item de la fuente nueva

#### Scenario: Referencia ausente, incorrecta o cuenta distinta
- **WHEN** falta `o`, está malformado o no coincide con la fuente accesible de la sesión
- **THEN** se ofrece volver al catálogo sin completar automáticamente el origen ni mostrar contenido de otra cuenta


### Requirement: Entrada directa y recarga independientes de memoria
El sistema SHALL resolver la selección completa desde la ruta mediante consultas autenticadas por identidad, sin depender de listados paginados, estado de navegación ni playback previo. SHALL mantener carga bajo demanda y la generación de catálogo activa existente, sin reimportar ni recorrer el catálogo para buscar un ID.

#### Scenario: Refrescar cada destino
- **WHEN** una sesión válida carga o recarga directamente cualquiera de las rutas canónicas sin selección en memoria
- **THEN** Nginx sirve la SPA y esta muestra la ficha, serie/temporadas o reproductor correctos sin regresar silenciosamente a `/`

#### Scenario: Ficha y reproducción
- **WHEN** se abre una ficha de película o serie y después una ruta de reproducción
- **THEN** la ficha no crea playback; la ruta de reproducción crea uno autorizado para esa entrada y conserva progreso/reanudación e inicio manual según el contrato existente

### Requirement: Resolución autorizada y ausencia de contenido
El API SHALL ofrecer consulta puntual de item por fuente/tipo/ID reutilizando el catálogo activo y autorización de propietario. SHALL devolver 401 sin sesión, 404 indistinguible para fuente ajena/inexistente e item ausente, y fallo transitorio saneado cuando corresponda. La web SHALL validar la pertenencia del episodio a su serie y nunca sustituir automáticamente la fuente o selección solicitadas.

#### Scenario: Fuente o contenido eliminados
- **WHEN** la fuente se eliminó, no pertenece a la sesión o el item no existe en una generación activa disponible
- **THEN** se muestra contenido no disponible con regreso al catálogo, sin revelar datos de otras cuentas, buscar títulos similares ni iniciar otro vídeo

#### Scenario: Catálogo pendiente o red fallida
- **WHEN** todavía no existe generación activa o falla temporalmente una consulta
- **THEN** la web diferencia preparación de catálogo de fallo recuperable, mantiene la URL y ofrece reintento sin afirmar que el contenido fue eliminado

#### Scenario: Episodio de otra serie
- **WHEN** la ruta combina una serie válida con un episodio que no pertenece a ella
- **THEN** no se abre reproducción y se ofrece volver a la serie autorizada

### Requirement: Historial y contexto de navegación
La web SHALL sincronizar selección visible con rutas e historial, incluso al reutilizar componentes, y conservar filtros/página de la misma sesión/fuente. La ficha de serie SHALL admitir `?o=:origen&temporada=N`; el episodio SHALL conservar su temporada real para el regreso, corregir su descriptor tNeM mediante reemplazo y normalizar temporada de ficha inválida a la primera disponible sin añadir historial.

#### Scenario: Recorrido con filtros
- **WHEN** se abre ficha desde un catálogo filtrado, se reproduce y se usa Atrás/Adelante
- **THEN** cada entrada representa la selección visible correcta, el catálogo recupera filtros/página y no aparecen bucles de navegación ni aperturas duplicadas

#### Scenario: Salida sin contexto previo
- **WHEN** se entra directamente en una ficha o reproductor y se usa cerrar, Escape o salir
- **THEN** la ficha vuelve al catálogo del tipo, el reproductor de película vuelve a su ficha, el episodio vuelve a su serie/temporada y directo vuelve a su catálogo, sin depender de historial externo

#### Scenario: Cambio de fuente o sesión
- **WHEN** se reemplaza/elimina la fuente o termina/cambia la sesión
- **THEN** se limpian filtros, resultados y contexto privado anterior antes de resolver nuevo contenido

### Requirement: Ciclo seguro de reproducción durante navegación
La web SHALL conservar cierre de playback, liberación de lease, progreso VOD y cancelación de conversión al abandonar o sustituir una reproducción. SHALL cancelar/ignorar resoluciones obsoletas y cerrar un playback recibido después de abandonar su apertura. SHALL conservar controles, diagnósticos, accesibilidad y foco existentes.

#### Scenario: Cambio rápido y parámetros nuevos
- **WHEN** se navega de un contenido a otro antes o después de resolver su playback, también reutilizando el componente
- **THEN** solo la última selección puede quedar activa, se cierran recursos previos/tardíos y no quedan leases ni conversiones huérfanos

#### Scenario: Cerrar ficha con teclado
- **WHEN** se cierra una ficha con Escape o su control de cierre
- **THEN** la URL se corresponde con el destino de regreso y el foco vuelve al disparador existente o al contenido principal del destino

### Requirement: Recuperación de destino tras autenticación
La web SHALL conservar el destino interno solicitado, incluidos slugs, origen y temporada, durante login y resolverlo después de autenticarse, sin permitir redirecciones externas ni recurrencias a login. SHALL admitir solo gramáticas exactas de rutas españolas/legacy y queries permitidas sin duplicados. El enlace SHALL seguir requiriendo autorización por usuario/fuente y nunca conceder acceso por su posesión.

#### Scenario: Enlace sin sesión
- **WHEN** se abre una ruta de contenido sin sesión y se inicia sesión correctamente
- **THEN** se vuelve al destino solicitado y el API verifica su propiedad para la nueva sesión antes de mostrar datos o crear playback

#### Scenario: Destino manipulado u otra cuenta
- **WHEN** el destino de retorno contiene un esquema, URL externa, `//`, variante codificada equivalente o login recursivo, o el contenido pertenece a otra cuenta
- **THEN** se rechaza el retorno inseguro con destino local seguro, o se muestra contenido no disponible tras autorización, sin exponer la cuenta original

### Requirement: Compatibilidad de rutas y ejecución existente
La aplicación SHALL conservar `/`, `/login` y compatibilidad legacy mediante reemplazo de URL tras autorizar la fuente exacta, sin sustituirla ni abrir doble playback. SHALL reutilizar Compose, variables y esquema existentes, sin migraciones nuevas, y conservar proxy API y assets.

#### Scenario: URL antigua y fallback SPA
- **WHEN** se abre `/player` sin memoria o se solicita una ruta profunda mediante HTTP GET
- **THEN** la primera vuelve al catálogo con aviso si no hay selección, o reemplaza a canónica adoptando el playback válido en memoria; nunca adivina contenido y la segunda permite a Angular resolver la selección; `/api/` conserva su contrato y no se trata como ruta de contenido

#### Scenario: Ruta técnica anterior
- **WHEN** se abre una URL `/sources/:sourceId/...` anterior con fuente/item aún autorizados
- **THEN** se reemplaza por la URL española con slug/origen y se conserva destino/contexto, traduciendo `season` a `temporada`; si la fuente fue sustituida se muestra no disponible

#### Scenario: Descriptor de episodio y sufijo inequívoco
- **WHEN** un título termina en números o el descriptor tNeM no coincide con el episodio del ID
- **THEN** el último guion delimita el ID decimal exacto, y el descriptor se corrige desde los datos del episodio autorizado sin seleccionar otro por temporada/número
