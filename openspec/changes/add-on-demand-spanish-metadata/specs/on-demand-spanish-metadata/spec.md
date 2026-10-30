# Spec Delta

## Purpose

Mostrar detalles fiables en español de obras IPTV abiertas por el usuario, con identidad conservadora, caché PostgreSQL y reproducción independiente de TMDB.

## ADDED Requirements

### Requirement: Consulta explícita y limitada a contenido propio
El sistema SHALL enriquecer únicamente fichas y temporadas propias abiertas explícitamente, previa autorización contra el catálogo activo. SHALL consultar solo datos mínimos del título y conservar privacidad entre usuarios. Listados, sincronización, arranque y reproducción SHALL no generar consultas TMDB.

#### Scenario: Condiciones de demanda y privacidad
- **WHEN** se implementa y verifica este requisito
- **THEN** El sistema SHALL consultar TMDB solo al abrir explícitamente una ficha de película/serie propia o una temporada seleccionada. SHALL autorizar propietario, fuente, tipo e item contra el catálogo activo antes de leer caché, resolver o llamar a red. La consulta SHALL contener solo título mínimo normalizado, año conocido e IDs externos válidos; nunca categoría privada, ID IPTV, fuente, URL, credenciales o identidad de usuario. Abrir listados, buscar, sincronizar, reiniciar y reproducir SHALL producir cero peticiones TMDB. El cliente SHALL informar en la ficha de la procedencia TMDB y la consulta externa de título.

#### Scenario: Catálogo grande y primera apertura
- **WHEN** se importa o lista un catálogo de 210.381 películas y 45.620 series
- **THEN** no se emite ninguna consulta TMDB
- **WHEN** se abre una película propia sin caché
- **THEN** se consulta solo esa obra, sin recorrer páginas del catálogo ni obras relacionadas

#### Scenario: Fuente ajena o item inexistente
- **WHEN** B solicita metadata usando fuente/item de A o un item ausente del catálogo activo
- **THEN** recibe 404 sin cache hit observable, títulos de A ni petición externa

### Requirement: Identidad conservadora y separada de reproducción
El sistema SHALL asociar obras solo con identidad externa verificada o un candidato único del mismo tipo con título exacto normalizado y año conocido coincidente. SHALL conservar decisiones privadas por huella del item, sin alterar recursos ni permisos IPTV.

#### Scenario: Reglas verificables de asociación
- **WHEN** se implementa y verifica este requisito
- **THEN** El sistema SHALL resolver un ID externo TMDB/IMDb solo si proviene de datos estructurados válidos del proveedor y verifica tipo y coherencia de título/año disponible en TMDB. Sin ID verificable SHALL aceptar búsqueda automática únicamente si existe un candidato único del mismo tipo con título u original normalizado exacto y año conocido coincidente. La normalización SHALL retirar solo etiquetas reconocidas de calidad/idioma y año final, conservar palabras significativas y no usar coincidencia difusa como prueba. Sin año, múltiples candidatos, resultados truncados/no exhaustivos o contradicción SHALL dejar estado `unresolved` o `ambiguous`, sin elegir el primer resultado. Un resultado de búsqueda por sí solo SHALL no acreditar disponibilidad IPTV.
- **AND** La decisión privada SHALL identificarse por propietario/fuente/tipo/ID IPTV y huella de título/año/ID externo, independiente de generación de importación. Un cambio de huella SHALL invalidarla; refrescar el mismo catálogo SHALL conservarla. Varias calidades verificadas de la misma obra SHALL compartir la caché canónica sin unir IDs, extensiones, permisos o streams. Borrar fuente SHALL borrar decisiones asociadas; un trabajo tardío SHALL no recrearlas.

#### Scenario: Remake y calidad
- **WHEN** dos películas comparten título pero años distintos y el item tiene año 1999
- **THEN** solo puede enlazarse el candidato exacto de 1999; sin año se conserva IPTV sin asociación
- **WHEN** dos calidades verificadas corresponden al mismo ID TMDB
- **THEN** comparten metadata y conservan recursos de reproducción distintos

#### Scenario: Coincidencia insuficiente y cambio de catálogo
- **WHEN** aparecen varios candidatos válidos, difiere el tipo/año o falta evidencia suficiente
- **THEN** no se guarda asociación positiva y la UI conserva IPTV sin inventar sinopsis
- **WHEN** el proveedor reutiliza un ID IPTV para otro título
- **THEN** la huella invalida la decisión anterior y no se muestra metadata de la obra previa

### Requirement: Ficha española y temporadas bajo demanda
El backend SHALL obtener metadata normalizada en `es-ES`, con fallback IPTV para campos ausentes y arte en host TMDB fijo. SHALL enriquecer solo la temporada seleccionada y episodios existentes en IPTV, sin crear disponibilidad ni recursos de reproducción desde TMDB.

#### Scenario: Campos localizados y correspondencia de temporadas
- **WHEN** se implementa y verifica este requisito
- **THEN** El backend SHALL solicitar `language=es-ES` y guardar campos normalizados acotados: ID/tipo, título localizado y original, sinopsis, fecha/año, géneros, duración cuando disponible, valoración con procedencia y créditos limitados a 20 personas. Campos ausentes SHALL omitirse; una sinopsis vacía SHALL no reemplazarse silenciosamente por inglés ni generarse. La UI SHALL usar campos IPTV como fallback y distinguir metadatos TMDB de idioma de audio inferido del proveedor. Si hay arte TMDB, SHALL aceptar solo rutas relativas de imagen del proveedor y construir URL en host HTTPS TMDB fijo, sin token; no admitir URLs arbitrarias ni cambiar el proxy privado IPTV.
- **AND** Temporadas SHALL consultarse únicamente al seleccionar una temporada existente de la serie; episodios TMDB SHALL enlazarse visualmente solo por número de temporada/episodio presente en IPTV. La lista IPTV SHALL seguir determinando episodios disponibles y sus IDs/extensiones, incluidos especiales de temporada 0. No se solicitarán todas las temporadas ni se crearán episodios reproducibles desde TMDB.

#### Scenario: Español incompleto y temporada elegida
- **WHEN** TMDB devuelve sinopsis española vacía y la serie tiene diez temporadas
- **THEN** se conserva texto IPTV o ausencia honesta sin mostrar inglés como español
- **WHEN** se abre la temporada 2
- **THEN** se solicita solo esa temporada; episodios adicionales de TMDB no añaden acciones de reproducción

### Requirement: Caché persistente con caducidad y degradación
El sistema SHALL persistir metadata pública por clave canónica/idioma y decisiones privadas por fuente/item. SHALL responder sin red desde caché fresca, servir stale acotado con refresco único y aplicar caché negativa solo a resultados semánticos. SHALL autorizar antes de acceder a cualquier caché.

#### Scenario: Caducidad y aislamiento persistentes
- **WHEN** se implementa y verifica este requisito
- **THEN** PostgreSQL SHALL almacenar fichas canónicas por `(provider=tmdb, kind, canonical_id, language=es-ES)` y temporadas por la misma clave más número de temporada. SHALL guardar respuesta normalizada versionada, `fetched_at` y expiración, sin payload raw privado. La caché positiva SHALL durar 7 días y podrá mostrarse stale hasta 30 días desde `fetched_at` mientras se refresca bajo demanda. Pasados 30 días SHALL usarse IPTV hasta obtener metadata fresca. Decisiones `unresolved`/`ambiguous` SHALL durar 24 horas por huella y evitar búsquedas repetidas. Errores de transporte, autenticación, 429 y 5xx SHALL no convertirse en negativos semánticos ni borrar metadata previa.
- **AND** Petición fresca SHALL responder desde PostgreSQL sin red incluso tras reinicio. Lectura stale SHALL devolver metadata inmediatamente y lanzar un solo refresco. La caché compartida SHALL contener únicamente datos públicos TMDB; asociaciones privadas y respuestas HTTP de fichas SHALL no compartirse entre usuarios ni almacenarse en caché pública. Fuente eliminada SHALL no dejar relaciones privadas; metadata pública canónica puede conservarse.

#### Scenario: Reinicio, stale y negativo
- **WHEN** una ficha fresca persistida se abre tras reiniciar API
- **THEN** no se llama a TMDB
- **WHEN** dos aperturas leen una ficha de 8 días
- **THEN** reciben stale sin esperar y generan un único refresco
- **WHEN** se repite una resolución sin coincidencia antes de 24 horas
- **THEN** no se repite la búsqueda; después del plazo puede intentarse bajo demanda

### Requirement: Recursos acotados y fallos externos
El adaptador SHALL acotar red, concurrencia, tasa, cola y reintentos; deduplicar por clave y respetar enfriamientos persistentes de 429. SHALL devolver IPTV/stale ante fallos sin borrar metadata previa ni exponer secretos. SHALL no ejecutar trabajos periódicos globales.

#### Scenario: Límites y enfriamientos comprobables
- **WHEN** se implementa y verifica este requisito
- **THEN** El adaptador SHALL usar HTTPS a hosts TMDB fijos, autenticación Bearer solo servidor, sin redirects a hosts arbitrarios, con timeout máximo de 5 segundos y respuesta máxima de 2 MiB por request. SHALL limitar a 2 peticiones simultáneas globales, 4 requests por segundo globales y 10 aperturas frías por minuto por usuario; excesos SHALL devolver estado temporal seguro con IPTV/stale. SHALL deduplicar resolución por clave privada y detalle/temporada por clave canónica antes de encolar. La cola SHALL admitir como máximo 32 trabajos pendientes; el exceso SHALL no crear goroutines/trabajos ilimitados. No habrá reintentos en bucle ni trabajo periódico global.
- **AND** En 429 SHALL respetar `Retry-After` válido (segundos o fecha), persistiendo la siguiente fecha permitida para sobrevivir reinicios. Ausencia/invalidez SHALL usar 60 segundos. Un valor largo SHALL no acortarse. Errores red/5xx SHALL enfriarse 30 segundos por clave; 401/403 SHALL enfriar el adaptador 5 minutos e indicar indisponibilidad saneada. Al vencer el enfriamiento SHALL permitirse un nuevo intento bajo demanda, sin reintento automático ilimitado. Respuestas externas y logs SHALL no incluir token, cuerpos raw, cabeceras sensibles ni consultas privadas completas.

#### Scenario: 429 y saturación
- **WHEN** TMDB devuelve 429 con `Retry-After: 120` y se reinicia API
- **THEN** no se vuelve a llamar al proveedor antes de la fecha persistida y permanece reproducción con IPTV/stale
- **WHEN** se reciben muchas aperturas simultáneas o TMDB tarda más de 5 segundos
- **THEN** se respetan concurrencia, tasa y cola acotadas; se expone estado temporal sin bloquear navegación ni playback

### Requirement: UX y contrato compatibles
La web SHALL mostrar inmediatamente la ficha IPTV y permitir playback durante todo estado de metadata. El enriquecimiento SHALL usar contrato aditivo independiente y descartar respuestas antiguas. SHALL mostrar procedencia, atribución TMDB y fallback honesto.

#### Scenario: Comportamiento de modal y contrato
- **WHEN** se implementa y verifica este requisito
- **THEN** La web SHALL abrir la ficha con datos IPTV ya disponibles y controles de reproducción utilizables antes de recibir metadata. La llamada metadata SHALL ser independiente de listar episodios IPTV o crear playback. Estados `loading`, `ready`, `stale`, `unresolved`, `ambiguous`, `disabled` y `unavailable` SHALL tener representación honesta y discreta; fallos no sustituirán la ficha por pantalla vacía. Cerrar/cambiar ficha o temporada SHALL cancelar suscripciones y descartar respuestas antiguas. El contrato nuevo SHALL ser aditivo y no cambiar rutas/payloads obligatorios de catálogo, biblioteca, episodios ni playback. UI SHALL mostrar atribución TMDB con su logo y aviso oficial de ausencia de respaldo/certificación, según sus condiciones.

#### Scenario: Playback durante fallo y respuesta antigua
- **WHEN** el enriquecimiento está pendiente, deshabilitado, devuelve error o no identifica la obra
- **THEN** película y episodios IPTV pueden iniciar reproducción con el flujo vigente y sin lease extra de metadata
- **WHEN** la ficha A termina de cargar después de abrir B
- **THEN** no sustituye título, imagen, sinopsis ni episodios de B

### Requirement: Configuración y migración aditivas
El sistema SHALL usar un token TMDB opcional solo en API y migración 004 aditiva ejecutada antes de API con el migrador existente. SHALL conservar el funcionamiento IPTV sin token y compatibilidad de la imagen anterior. Compose SHALL mantener servicios y PostgreSQL independientes, con una sola réplica.

#### Scenario: Operación y recuperación verificables
- **WHEN** se implementa y verifica este requisito
- **THEN** `TMDB_API_TOKEN` SHALL ser opcional y exclusivo del entorno runtime del servicio API. Ausencia SHALL deshabilitar llamadas y conservar arranque/health/playback IPTV; un token configurado inválido SHALL producir fallo saneado del adaptador. SHALL no llegar a web, build args, bundles, respuestas, `.env.example` real ni logs. `compose.yml` SHALL pasar solo el nombre de la variable a API; `.env.example` SHALL documentarla vacía, y setup local SHALL conservar `.env` y secretos existentes sin generar credenciales TMDB. No se añadirán servicios ni dependerá la migración del token. `API_REPLICAS=1` SHALL mantenerse por coordinación en memoria actual.
- **AND** Migración 004 SHALL crear tablas/índices/FK aditivos sin modificar 001–003 ni perder usuarios, fuentes, catálogo o biblioteca. El job existente `walactv migrate` SHALL aplicarla una sola vez bajo advisory lock y transacción antes de iniciar API, tanto con PostgreSQL local aislada como externa. Antes de aplicarla localmente SHALL obtenerse backup privado conservando `IPTV_ENCRYPTION_KEY`; producción exige backup/restore verificado y autorización. Imagen anterior SHALL seguir funcionando e ignorar tablas nuevas; rollback SHALL conservar esquema/caché y no ejecutar down/drop. Volver a imagen nueva SHALL reutilizar datos válidos. Esta tarea SHALL no publicar ni usar producción.

#### Scenario: Actualización repetida y rollback
- **WHEN** se ejecutan dos migradores con 004 sobre una base 003 con datos
- **THEN** se registra una única versión y usuarios/fuentes/catálogo/biblioteca permanecen intactos
- **WHEN** se arranca una imagen previa y después la nueva
- **THEN** las tablas añadidas permanecen y los flujos anteriores siguen siendo compatibles

#### Scenario: Token ausente
- **WHEN** Compose inicia sin `TMDB_API_TOKEN`
- **THEN** los servicios existentes arrancan saludables, metadata indica `disabled` y la reproducción IPTV funciona
