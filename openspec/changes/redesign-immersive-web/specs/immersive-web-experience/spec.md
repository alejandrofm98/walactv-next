# Spec Delta

## ADDED Requirements

### Requirement: Identidad visual propia
La web SHALL conservar identidad propia en marca, navegación y catálogo, sin reutilizar marcas, recursos gráficos, iconografía distintiva ni textos de Nuvio/Stremio. El reproductor VOD SHALL poder inspirarse en sus affordances y composición inmersiva. El catálogo SHALL priorizar un encabezado compacto y controles directos de la biblioteca IPTV personal.

#### Scenario: Diferenciar la biblioteca
- **WHEN** se abre Directo, Películas o Series
- **THEN** se ve el nombre claro de la sección, los controles de catálogo sin un banner promocional genérico ni elementos que imiten otra plataforma

### Requirement: Lenguaje visual inmersivo coherente
La web SHALL usar el mismo sistema documentado de tokens OKLCH, tipografía legible, espaciado, controles y estados en acceso, shell, conexión de fuente, directo, películas, series, favoritos, Ace Stream, diálogos y reproductor. SHALL mantener tema oscuro, idioma español y registro de producto centrado en descubrir y reproducir contenido.

#### Scenario: Recorrido completo
- **WHEN** se recorre acceso, biblioteca, ficha y reproductor
- **THEN** acciones equivalentes conservan estilo/nombre/estados coherentes y el contenido y la acción principal tienen mayor jerarquía que la decoración

### Requirement: Representación honesta del contenido
La web SHALL presentar logos de directo sin recorte que impida reconocerlos, carátulas VOD con proporción estable y fallback legible cuando no exista imagen o falle. SHALL conservar procedencia y ausencia de metadata sin inventar carteles, títulos destacados, recomendaciones o capacidades.

#### Scenario: Imagen ausente y título largo
- **WHEN** un canal, película o serie no tiene imagen o tiene nombre largo
- **THEN** su espacio reservado no colapsa, el elemento sigue identificable y la acción accesible conserva el nombre completo

### Requirement: Estructura adaptable completa
La web SHALL ofrecer todos los destinos y acciones actuales en escritorio y móvil sin depender de hover. A 320 CSS px SHALL evitar scroll horizontal del documento y a zoom 200% SHALL conservar acceso a navegación, campos, acciones y cierre de diálogos. Los controles principales SHALL tener área táctil mínima de 44×44 CSS px.

#### Scenario: Catálogo y ficha en móvil
- **WHEN** se usa navegación, búsqueda, categoría, idioma, paginación y ficha a 390×844 y a 320px de ancho
- **THEN** las etiquetas/controles son legibles, no se solapan y se puede alcanzar la acción principal y cerrar/volver

### Requirement: Accesibilidad y movimiento básico
La web SHALL conservar landmarks, jerarquía de headings, etiquetas, nombres accesibles y navegación por teclado. Texto normal SHALL alcanzar contraste 4.5:1, texto grande 3:1 y foco visible 3:1 contra superficie adyacente con indicador de al menos 2px. SHALL respetar movimiento reducido y no comunicar estado únicamente mediante color.

#### Scenario: Ficha con teclado
- **WHEN** se abre una ficha o confirmación por teclado y se recorre con Tab
- **THEN** el foco entra y permanece en el diálogo, se ve sobre todos los fondos y vuelve al disparador al cerrar si aún existe; Escape permite cerrarlo cuando no hay operación que lo impida

#### Scenario: Preferencia de movimiento reducido
- **WHEN** el dispositivo comunica `prefers-reduced-motion: reduce`
- **THEN** se elimina movimiento/shimmer no esencial sin ocultar carga, selección o errores

### Requirement: Estados accionables y seguros
La web SHALL distinguir carga, recuperación de fuente, sincronización, vacío, ausencia de resultados, fallo y estado deshabilitado. SHALL mostrar error y reintento donde proceda, conservar confirmación destructiva y no mostrar credenciales, URLs privadas ni diagnósticos sin sanear.

#### Scenario: Sin resultados o fallo de actualización
- **WHEN** una búsqueda no devuelve resultados o falla una actualización conservando catálogo previo
- **THEN** se presenta la causa correspondiente y una acción pertinente sin confundirla con falta de fuente ni impedir explorar la copia conservada

### Requirement: Conservar contratos y navegación de reproducción
El rediseño SHALL conservar rutas, contratos de API, filtros/búsqueda/categoría/idioma/página al regresar de reproducción, limpieza al cambiar fuente/sesión, favoritos, temporadas/episodios, metadata, apertura Ace Stream y lifecycle/controles/diagnósticos del reproductor existentes. SHALL mantener carga bajo demanda, paginación y lazy loading de arte sin añadir llamadas de datos o reproducción automática decorativas.

#### Scenario: Volver con filtros
- **WHEN** se aplica búsqueda, categoría, idioma y página, se abre un título y se vuelve desde su reproducción
- **THEN** se conserva el estado de catálogo de la sesión y fuente vigentes y salir libera la reproducción según el contrato actual

#### Scenario: Reproductor con vídeo incompatible
- **WHEN** la reproducción necesita inicio manual, comunica error o usa conversión compatible
- **THEN** la nueva presentación conserva las acciones y estados existentes sin ocultar controles ni atribuir una causa que el diagnóstico no confirma


### Requirement: Filtros en orden estable
La web SHALL presentar búsqueda, idioma y categoría en ese orden en cada catálogo que ofrezca dichos filtros. El orden visual, DOM y recorrido de teclado SHALL coincidir en escritorio y móvil. Reordenar SHALL conservar valores, búsqueda y persistencia actuales.

#### Scenario: Idioma antes de categoría
- **WHEN** se abre Directo, Películas o Series en escritorio o móvil
- **THEN** el selector Idioma aparece y recibe foco antes que Categoría, manteniendo sus valores al reproducir y volver

### Requirement: Microcopy concisa y funcional
La web SHALL eliminar eslóganes, explicaciones obvias, subtítulos repetidos y estados decorativos de cuenta/dispositivo/sesión en acceso, biblioteca, fichas y reproductor. SHALL conservar instrucciones necesarias, labels, privacidad, compatibilidad, atribuciones, estados y errores accionables.

#### Scenario: Acción sin texto redundante
- **WHEN** se abre acceso, conexión de fuente o reproducción
- **THEN** la acción principal y sus etiquetas bastan para operar, sin frases promocionales ni dos controles equivalentes de salida; se mantienen ayuda necesaria y errores/reintentos

### Requirement: Presentación de reproducción según contenido
El reproductor SHALL distinguir directo y VOD usando el tipo existente. Directo SHALL priorizar canal e indicador de emisión con ergonomía IPTV; VOD SHALL priorizar vídeo y controles discretos inspirados en Nuvio/Stremio. SHALL conservar controles, progreso, inicio manual, errores, fullscreen y lifecycle existentes, sin barras duplicadas ni capacidades no disponibles.

#### Scenario: Canal en directo
- **WHEN** se reproduce un elemento `live`
- **THEN** el vídeo, título de canal, estado En directo, salida y controles actuales son accesibles, sin duración/progreso VOD inventados, conversión VOD, parrilla, zapping ni timeshift añadido

#### Scenario: Película o episodio
- **WHEN** se reproduce una película o episodio, directamente o con conversión existente
- **THEN** el vídeo domina la pantalla y se conserva progreso/reanudación VOD; la búsqueda de conversión solo aparece con duración fiable, y no se añaden selectores de fuentes, audio, subtítulos ni navegación de episodios sin datos/capacidad existentes

#### Scenario: Salir desde ambos modos
- **WHEN** se usa la salida accesible o la navegación existente desde directo o VOD
- **THEN** se libera la reproducción y su lease, VOD guarda el progreso conforme al contrato existente y el catálogo recupera los filtros previos
