# Spec Delta

## Purpose

Conservar un catálogo IPTV privado completo y consultable tras reinicios, con sincronización controlada y descubrimiento por nombre, categoría e idioma.

## ADDED Requirements

### Requirement: Catálogo completo privado persistente
El sistema SHALL guardar categorías y contenidos live/movie/series completos de la fuente propia en PostgreSQL y recuperarlos tras reinicio sin volver a introducir URL/credenciales. SHALL consultar solo generación activa propia, conservar cifrado de fuente y revalidar cuenta antes de reproducción según contrato existente.

#### Scenario: Reinicio y cuenta distinta
- **WHEN** A sincroniza una fuente y reinicia API
- **THEN** recupera categorías, títulos y contadores guardados sin acceder al proveedor para listar
- **WHEN** B usa el ID de fuente de A
- **THEN** recibe 404 sin títulos, imágenes ni estado de A

### Requirement: Sincronización controlada y atómica
El sistema SHALL ofrecer inicio asíncrono, estado, fecha y reintento de sincronización con límites del contrato. SHALL deduplicar por fuente/cuenta, consultar secuencialmente y publicar categorías/items/contadores de los tres tipos en una única generación solo tras éxito completo. SHALL no abrir streams adicionales.

#### Scenario: Actualización concurrente y fallo parcial
- **WHEN** dos pestañas solicitan actualizar y el proveedor falla después del segundo tipo
- **THEN** existe un solo trabajo, se conserva completa la generación anterior y se informa fallo con opción de reintentar

#### Scenario: Sin generación inicial
- **WHEN** una fuente guardada carece de generación y se inicia sincronización
- **THEN** la UI muestra carga y la lista vacía hasta la publicación completa, sin exigir reintroducir la fuente

#### Scenario: Borrado o reinicio durante sincronización
- **WHEN** se elimina o reemplaza la fuente con trabajo pendiente
- **THEN** el trabajo no puede resucitar fuente ni catálogo
- **WHEN** el proceso reinicia durante trabajo
- **THEN** conserva la generación anterior y expone estado reintentable sin afirmar éxito

#### Scenario: Vídeo activo y límites
- **WHEN** se sincroniza mientras existe un stream activo o se supera tamaño/tiempo máximo
- **THEN** no se abre otro stream ni se cancela el activo y, ante exceso, se conserva el catálogo previo con error explícito

### Requirement: Búsqueda global combinable y paginación
El sistema SHALL buscar por nombre sin distinguir mayúsculas en todo el tipo seleccionado desde PostgreSQL, combinando categoría e idioma antes de contar/paginar y con orden determinista. Ausencia de categoría/idioma SHALL significar todos; caracteres de búsqueda SHALL tratarse como texto literal y valores inválidos SHALL rechazarse.

#### Scenario: Todas las categorías y filtros combinados
- **WHEN** se busca un nombre presente en dos categorías con categoryId omitido
- **THEN** aparecen ambas coincidencias con total correcto aunque no estuvieran consultadas previamente
- **WHEN** se añade categoría e idioma
- **THEN** resultados y total satisfacen los tres filtros y la UI reinicia página

#### Scenario: Entrada y respuestas fuera de orden
- **WHEN** se usan `%` o `_` en búsqueda o una petición antigua termina después de la vigente
- **THEN** la búsqueda conserva significado literal y la UI no sustituye los resultados vigentes por los antiguos

### Requirement: Etiquetado de idioma honesto
El sistema SHALL inferir idioma solo de etiquetas explícitas de título/categoría y conservar `und` cuando no hay evidencia. SHALL mostrar «Desconocido» y aclarar que el filtro representa etiqueta proveedor, sin prometer pista de audio.

#### Scenario: Contenido sin etiqueta
- **WHEN** un item no contiene etiqueta reconocible ni categoría con idioma explícito
- **THEN** aparece como Desconocido y puede filtrarse con `language=und`

### Requirement: Migración compatible
El sistema SHALL aplicar migración aditiva 003 una vez bajo lock antes de iniciar API, conservar fuentes/biblioteca previas y usar la clave externa original. SHALL conservar las nuevas tablas al volver de imagen sin ejecutar borrado automático.

#### Scenario: Actualización y recuperación
- **WHEN** se aplica 003 dos veces a la base local con datos existentes
- **THEN** se aplica una sola versión, permanecen usuarios/fuentes/biblioteca y la generación puede llenarse por demanda
