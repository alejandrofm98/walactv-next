# Spec Delta

## Purpose

Permitir descubrir películas y series mediante Cinemeta mostrando únicamente correspondencias verificadas con el catálogo IPTV privado disponible del usuario.

## ADDED Requirements

### Requirement: Descubrimiento explícito solo para VOD
La aplicación SHALL ofrecer «Descubrir con Cinemeta» en Películas y Series, separado del catálogo IPTV completo. SHALL conservar Directos, favoritos, rutas de ficha y reproducción existentes sin filtros Cinemeta.

#### Scenario: Entrada voluntaria y exclusión de Directos
- **WHEN** se inicia sesión, sincroniza IPTV, abre el catálogo habitual o navega por Directos
- **THEN** no se consulta Cinemeta y el catálogo conserva su comportamiento actual
- **WHEN** el usuario activa descubrimiento de películas o series
- **THEN** ve atribución Cinemeta y una explicación de la cobertura por IMDb; puede regresar al catálogo completo

### Requirement: Identidad IMDb conservadora
El sistema SHALL unir exclusivamente IMDb IDs estructurados válidos e iguales del mismo tipo entre Cinemeta e IPTV. SHALL excluir contradicciones conocidas de año/tipo y conservar sin asociación los elementos sin IMDb. SHALL no resolver automáticamente por semejanza de título ni consultar TMDB desde descubrimiento.

#### Scenario: Identidad frente a remakes y títulos localizados
- **WHEN** Cinemeta e IPTV tienen el mismo IMDb y tipo, títulos en idiomas distintos y ningún año conocido contradictorio
- **THEN** se admite la correspondencia
- **WHEN** solo coincide el título, falta IMDb, difiere el IMDb, el tipo o el año conocido
- **THEN** no aparece en descubrimiento y permanece disponible en el catálogo IPTV habitual

#### Scenario: Varias versiones IPTV
- **WHEN** dos películas IPTV con distintas calidades comparten IMDb válido
- **THEN** se presentan sus versiones con identidad IPTV diferenciada y acceso a cada recurso original, sin duplicar una misma versión en páginas sucesivas

### Requirement: Disponibilidad y reproducción exclusivamente IPTV
Cada resultado SHALL pertenecer a la generación activa de la fuente autorizada y tener identidad IPTV utilizable por el flujo de reproducción vigente. Cinemeta SHALL no crear películas, episodios, URLs de vídeo ni permisos. La disponibilidad SHALL significar presencia IPTV, sin garantizar que el proveedor entregue el flujo en ese momento.

#### Scenario: Obra externa ausente y serie propia
- **WHEN** Cinemeta devuelve una obra ausente del catálogo IPTV del usuario
- **THEN** la obra no se muestra ni ofrece reproducción
- **WHEN** una serie propia aparece en descubrimiento
- **THEN** abre sus temporadas y episodios IPTV mediante el flujo existente; solo los episodios IPTV autorizados pueden reproducirse y Cinemeta no añade episodios

#### Scenario: Catálogo modificado antes de reproducir
- **WHEN** una obra previamente mostrada desaparece de la generación activa o se elimina su fuente
- **THEN** una nueva consulta la excluye y el flujo de reproducción vuelve a autorizarla, sin usar disponibilidad externa ni permisos de una página antigua

### Requirement: Búsqueda y filtros con procedencia
El descubrimiento SHALL admitir catálogo Popular, género y búsqueda explícita Cinemeta, combinables con categoría e idioma aproximado IPTV mediante intersección en Go. SHALL validar filtros y tipos antes de red, mostrar etiquetas españolas y conservar los identificadores externos sin inferir idioma de audio desde Cinemeta.

#### Scenario: Combinación y validación
- **WHEN** se solicita una película de género Drama con búsqueda y categoría/idioma IPTV
- **THEN** solo se muestran correspondencias que cumplen los filtros externos y privados; categoría e idioma privados no se envían a Cinemeta
- **WHEN** se solicita un tipo live, género no permitido, búsqueda de más de 150 caracteres o cursor manipulado
- **THEN** se devuelve 400 sin petición externa

### Requirement: Paginación incremental honesta y acotada
El sistema SHALL paginar después de intersectar, conservando orden Cinemeta, versiones IPTV y continuidad sin omisiones por cortar una página externa. SHALL devolver como máximo 40 versiones por respuesta y un cursor de continuación privado. SHALL distinguir fin externo, exploración parcial y fallo temporal sin presentar el total Cinemeta como total IPTV.

#### Scenario: Página dispersa y buffer pendiente
- **WHEN** tres páginas externas consecutivas no contienen correspondencias
- **THEN** la respuesta puede estar vacía pero indica exploración parcial y permite continuar mientras exista continuidad externa
- **WHEN** una página externa produce más de 40 versiones coincidentes
- **THEN** las restantes se conservan para la siguiente página sin saltarlas ni repetir versiones ya entregadas

#### Scenario: Cambio de consulta o catálogo
- **WHEN** cambian filtros o tipo
- **THEN** se reinicia la continuación
- **WHEN** cambia la generación activa, expira el cursor o reinicia API
- **THEN** se indica reinicio requerido y la UI ofrece recomenzar sin mezclar resultados de generaciones distintas

### Requirement: Aislamiento privado previo a caché y red
El sistema SHALL autorizar sesión y propiedad de fuente antes de cualquier consulta o acceso a caché. SHALL vincular cursores y resultados a usuario, sesión, fuente, generación y filtros. SHALL no compartir respuestas privadas, credenciales, catálogo IPTV ni asociaciones entre usuarios.

#### Scenario: Dos usuarios y sesión revocada
- **WHEN** B usa una fuente o cursor de A
- **THEN** no recibe títulos, conteos ni señales de caché privados de A; fuente ajena responde 404 y cursor ajeno se rechaza sin red
- **WHEN** se cierra sesión o elimina la fuente con una petición en curso
- **THEN** se descarta su respuesta y los cursores privados quedan inutilizables

#### Scenario: Privacidad de solicitudes externas
- **WHEN** el usuario escribe y envía una búsqueda de descubrimiento
- **THEN** Cinemeta recibe solo tipo, búsqueda explícita, género público y desplazamiento; nunca IDs IPTV, usuario, fuente, categoría privada, idioma privado, credenciales ni historial

### Requirement: TMDB español y degradación independiente
Las fichas SHALL seguir usando TMDB `es-ES` bajo demanda y fallback IPTV según el contrato vigente. Cinemeta SHALL aportar clasificación y descubrimiento con atribución, sin sustituir detalles españoles ni hacer que playback dependa de metadata externa. Fallos externos SHALL preservar el catálogo habitual y mensajes españoles recuperables.

#### Scenario: Fallo de Cinemeta y detalles localizados
- **WHEN** Cinemeta falla o no cubre una obra
- **THEN** se ofrece volver al catálogo IPTV sin borrar filtros de este, sin impedir reproducción y sin confundir fallo con ausencia de obras
- **WHEN** se abre una ficha encontrada
- **THEN** se utiliza su identidad IPTV para pedir los detalles TMDB existentes; sinopsis inglesa Cinemeta no sustituye sinopsis española

### Requirement: Límites externos y operación compatible
El adaptador SHALL usar HTTPS y hosts Cinemeta fijos permitidos, timeout máximo 5 segundos, respuesta máxima 2 MiB, dos solicitudes simultáneas y cuatro solicitudes por segundo globales. SHALL limitar a tres páginas externas por respuesta y diez solicitudes de descubrimiento por minuto por usuario, sin colas ilimitadas ni reintentos automáticos.

#### Scenario: Saturación y enfriamiento
- **WHEN** hay timeout, exceso de límites, JSON inválido o 5xx
- **THEN** se devuelve indisponibilidad temporal saneada y no se consume una continuación irrecuperable; errores de transporte se enfrían 30 segundos
- **WHEN** se recibe 429
- **THEN** se respeta Retry-After válido sin acortarlo, o 60 segundos si falta, y no se reintenta en bucle

#### Scenario: Compatibilidad operativa
- **WHEN** se instala o revierte la imagen del cambio en el entorno local aislado
- **THEN** no son necesarias variables, secretos, servicios, volúmenes ni migraciones nuevas; el migrador existente no tiene versiones adicionales y PostgreSQL permanece intacta
- **WHEN** se ejecutan comprobaciones de salud sin conectividad Cinemeta
- **THEN** el arranque y health siguen correctos; se conserva API_REPLICAS=1 y PostgreSQL productiva externa a Compose

### Requirement: Estado de navegación privado
La web SHALL conservar modo, tipo, filtros, páginas ya visitadas y foco al volver de ficha o playback en la misma sesión/fuente. SHALL cancelar peticiones anteriores al cambiar de contexto y limpiar estado privado al terminar sesión o cambiar/eliminar fuente.

#### Scenario: Regreso y respuesta tardía
- **WHEN** se vuelve de una ficha o reproducción en la misma sesión/fuente
- **THEN** se restaura descubrimiento con sus filtros y página, revalidando la generación antes de usar una continuación
- **WHEN** llega una respuesta de la búsqueda anterior después de cambiar filtros o usuario
- **THEN** no reemplaza resultados ni estado del contexto nuevo
