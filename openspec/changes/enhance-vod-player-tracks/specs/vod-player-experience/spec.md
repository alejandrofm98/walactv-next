# Spec Delta

## Purpose

Ofrecer reproducción inmersiva de películas y episodios con controles accesibles y elección de audio y subtítulos limitada a las pistas que el medio y el navegador permiten usar realmente.

## ADDED Requirements

### Requirement: Composición inmersiva VOD propia
El reproductor VOD SHALL priorizar vídeo, título, una salida y controles discretos con los tokens oscuros y latón cálido de WalacTV, sin copiar marcas o recursos distintivos. SHALL ofrecer reproducir/pausar, progreso, volumen/silencio y fullscreen cuando estén soportados, sin barras duplicadas.

#### Scenario: Película y episodio reproducibles
- **WHEN** se abre una película o episodio reproducible directamente o con conversión
- **THEN** el vídeo domina el área disponible, el título identifica el contenido, una acción permite volver y los controles visibles representan el estado real de reproducción

#### Scenario: Controles discretos accesibles
- **WHEN** hay reproducción activa e inactividad de interacción durante tres segundos
- **THEN** los controles superpuestos se ocultan; reaparecen con movimiento, toque o teclado, y permanecen visibles durante pausa, menú abierto o foco dentro de controles, sin impedir las acciones al primer toque

### Requirement: Audio seleccionable real
El reproductor VOD SHALL ofrecer selección de audio solo con al menos dos pistas que el motor activo pueda enumerar y seleccionar. SHALL mostrar selección confirmada por el motor y etiquetas derivadas de idioma/nombre reales; a falta de ambos SHALL usar nombres neutrales numerados, sin inferir idiomas del catálogo o nombre del archivo.

#### Scenario: HLS con audio alternativo
- **WHEN** el HLS autorizado expone dos pistas de audio seleccionables y el usuario elige una
- **THEN** cambia el audio del mismo playback, se actualiza la selección al confirmarse, continúa la posición sin reiniciar a cero y no se crea otro playback ni lease

#### Scenario: Audio único o sin audio
- **WHEN** el motor expone cero o una pista de audio
- **THEN** no aparece un selector de audio; los controles de sonido conservan su comportamiento válido y no se promete otro idioma

### Requirement: Subtítulos disponibles y desactivables
El reproductor VOD SHALL ofrecer selección de subtítulos solo si el motor activo expone al menos una pista de subtítulos o captions representable. SHALL incluir Desactivados y permitir mostrar solo la pista elegida; SHALL excluir pistas de metadata y no consultar servicios externos ni fabricar pistas ausentes.

#### Scenario: HLS con subtítulos
- **WHEN** un HLS autorizado expone subtítulos compatibles y se selecciona una pista y después Desactivados
- **THEN** se muestran sus cues durante la reproducción y después dejan de mostrarse, reflejando ambas selecciones sin reiniciar el vídeo

#### Scenario: Medio sin subtítulos utilizables
- **WHEN** el motor no expone pistas de subtítulos representables
- **THEN** no aparece un selector de subtítulos vacío o una opción de activarlos; la información de pistas indica que no hay subtítulos disponibles en esta reproducción

### Requirement: Compatibilidad nativa honesta
Para archivos y HLS nativos, el reproductor VOD SHALL usar únicamente pistas enumerables y controlables por el navegador. Cuando no exista API utilizable SHALL conservar reproducción y controles nativos que sean necesarios, y explicar en información de pistas que la selección no está disponible en ese navegador sin afirmar que el archivo carece de pistas.

#### Scenario: Archivo con pistas expuestas
- **WHEN** un archivo nativo expone dos pistas de audio controlables o una pista de subtítulos representable
- **THEN** se ofrece la selección correspondiente con los mismos criterios y estado real de HLS

#### Scenario: Archivo o HLS nativo sin API utilizable
- **WHEN** el navegador reproduce el medio pero no expone control de pistas
- **THEN** continúa la reproducción sin selectores falsos y la información de pistas distingue limitación del navegador de ausencia confirmada de pistas

### Requirement: Inventario acotado de archivos autorizado
El sistema SHALL poder inspeccionar pistas de archivos VOD autorizados cuando la conversión esté habilitada, a petición explícita, conservando el mismo lease. SHALL devolver identificadores, tipo, idioma y disponibilidad saneados, sin URLs privadas ni metadata arbitraria; SHALL distinguir inspección incompleta, no soportada, ocupada y deshabilitada de ausencia de pistas.

#### Scenario: MP4 o MKV con pistas embebidas
- **WHEN** se solicita inspeccionar un archivo compatible con varias pistas
- **THEN** se pausa y drena la lectura vigente antes de inspeccionar, se obtiene un inventario limitado y se puede reanudar desde la posición guardada sin conexión adicional simultánea

#### Scenario: Inspección costosa o ajena
- **WHEN** se excede el presupuesto, el propietario es otro o no se puede inspeccionar la entrada
- **THEN** se rechaza o termina sin revelar medios, se liberan recursos y se permite continuar la reproducción anterior sin afirmar que no existen pistas

### Requirement: Audio de archivo seleccionable en conversión
El sistema SHALL permitir seleccionar audio descubierto y decodificable del archivo mediante conversión VOD habilitada, conservando posición lógica y controles. SHALL validar en servidor que el identificador corresponde al inventario vigente. La opción SHALL explicar que se prepara una versión compatible hasta 720p cuando el navegador no puede cambiar esa pista directamente.

#### Scenario: Elegir segundo audio de MKV
- **WHEN** el usuario confirma un audio descubierto diferente desde directo o conversión
- **THEN** se detiene y drena la lectura/trabajo anterior, se inicia una generación con ese audio en la posición lógica actual y se confirma al reproducirse sin crear otro lease

#### Scenario: Identificador manipulado o servicio ocupado
- **WHEN** se solicita audio de otro playback, índice no inventariado o el recurso global está ocupado
- **THEN** se rechaza con error accionable sin ejecutar un mapa arbitrario ni detener una reproducción válida antes de comprobar la solicitud

### Requirement: Subtítulos textuales embebidos compatibles
La conversión SHALL permitir seleccionar pistas descubiertas SubRip, ASS/SSA, mov_text o WebVTT convertibles a texto WebVTT, y desactivarlas. SHALL conservar sincronía con offset, pausa y seek, sin ejecutar otra lectura upstream concurrente ni extracción completa. SHALL explicar pérdida de estilos ASS y rechazar PGS/VobSub u otros formatos no soportados sin prometer OCR o pistas externas.

#### Scenario: SubRip o mov_text seleccionado
- **WHEN** se elige una pista textual soportada en un archivo
- **THEN** se prepara una generación compatible en la posición actual, los cues se muestran sincronizados, el seek renueva sus timestamps y Desactivados elimina inmediatamente los cues visibles

#### Scenario: Solo subtítulos de imagen o conversión deshabilitada
- **WHEN** el inventario contiene PGS/VobSub o la conversión está deshabilitada
- **THEN** no se ofrece activar esa pista como texto y se explica el límite correspondiente; no se inventan subtítulos ni se inicia una conversión oculta

### Requirement: Presupuestos y privacidad de pistas de archivo
La inspección y conversión con pistas SHALL compartir el slot global VOD sin cola y los límites existentes de CPU, memoria, salida e inactividad. SHALL usar gateway loopback, procesos sin URL privada y assets por propietario/playback/generación con no-store. SHALL cancelar y eliminar inventario/cues efímeros al revocar reproducción, fuente o sesión.

#### Scenario: Inspección y reproducción concurrentes
- **WHEN** otra conversión ocupa el slot o hay lecturas activas del archivo a inspeccionar
- **THEN** se informa ocupado o se drenan las lecturas antes de abrir el gateway; no hay dos entradas upstream simultáneas del trabajo ni se amplían los recursos

#### Scenario: Cue o generación ajenos
- **WHEN** se piden cues de otra cuenta o una generación revocada
- **THEN** no se recibe contenido ni se abre conexión upstream y no quedan cues previos tras seek o salida

### Requirement: Lifecycle y fallos de pistas seguros
El reproductor VOD SHALL refrescar opciones al cambiar las pistas del motor y limpiarlas al cambiar medio, intento o generación y al salir. SHALL conservar selección válida dentro del mismo intento sin reutilizar índices obsoletos. Los fallos de una pista SHALL mostrarse sin confirmar selección fallida ni crear bucles de reintento o conversión para suplir idiomas.

#### Scenario: Pistas tardías o desaparecidas
- **WHEN** llega una lista de pistas tras cargar metadata, cambia el grupo HLS o desaparece la pista elegida
- **THEN** se actualizan opciones y selección efectiva; no queda una opción seleccionada inexistente ni se selecciona otro idioma por un índice anterior

#### Scenario: Falla el cambio de pista
- **WHEN** una pista seleccionada falla al cargar o el motor rechaza el cambio
- **THEN** se muestra un error conciso, se conserva o recupera la pista anterior si el motor lo permite, y un fallo fatal mantiene las acciones existentes de recuperación

#### Scenario: Salida o nueva reproducción
- **WHEN** se vuelve al catálogo o se abre otro contenido
- **THEN** se liberan listeners y motor del intento anterior, no aparecen pistas del contenido previo y se conserva guardado de progreso, liberación de lease y restauración de filtros

### Requirement: Accesibilidad y fullscreen VOD
Los controles VOD SHALL ser operables con teclado y toque, tener nombre accesible, foco visible y objetivos de 44×44 CSS px. SHALL funcionar a 320, 390, 768 y 1440 px y zoom 200%, sin scroll horizontal del documento. Los menús SHALL ser alcanzables dentro de fullscreen cuando el navegador permita fullscreen del contenedor.

#### Scenario: Menú con teclado y móvil
- **WHEN** se abre el menú de pistas con teclado o toque en móvil o fullscreen
- **THEN** las opciones, selección y cierre son legibles y accesibles, Escape cierra el menú y devuelve foco al disparador sin activar a la vez salida de reproducción

#### Scenario: Fullscreen limitado y movimiento reducido
- **WHEN** solo existe fullscreen nativo de vídeo o se solicita movimiento reducido
- **THEN** se conservan los controles nativos necesarios y la selección ofrecida por el navegador, se puede acceder al menú propio al salir de fullscreen y se elimina movimiento no esencial

### Requirement: Conservar contratos y aislamiento de Live
Este cambio SHALL conservar autorización y URLs opacas, límites de conexiones, diagnóstico saneado, inicio manual, progreso/reanudación y conversión existentes. SHALL mantener Live sin cambios funcionales ni visuales. La selección de pistas SHALL usar el mismo playback autorizado sin nuevas conexiones de sondeo ni divulgar URLs privadas.

#### Scenario: Pistas HLS protegidas
- **WHEN** un manifiesto usa audio o subtítulos en playlists o segmentos separados
- **THEN** sus solicitudes usan assets opacos y validación/autorización existentes, un usuario ajeno no accede a ellas y los límites actuales se mantienen

#### Scenario: Canal en directo
- **WHEN** se reproduce un canal live después del cambio
- **THEN** conserva composición, controles y lifecycle previos, sin menús VOD ni cambios derivados de selección de pistas
