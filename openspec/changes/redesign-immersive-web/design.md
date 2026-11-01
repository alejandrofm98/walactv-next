# Design

## Context

La web Angular standalone tiene tres rutas cargadas bajo demanda: acceso, biblioteca y reproductor. La biblioteca contiene navegación Directo/Películas/Series/Mi lista/Ace Stream, fuente privada, sincronización, filtros, fichas y episodios. Los servicios de estado preservan los filtros al volver de reproducción. La API conserva credenciales y devuelve recursos autorizados; la UI nunca necesita mostrar una URL privada.

Dirección confirmada: registro `product`, cine en casa inmersiva y sin requisitos adicionales específicos del usuario. Se conserva accesibilidad básica. La identidad global y el catálogo deben ser propios. La preferencia posterior concreta una excepción para el reproductor de películas y series: tomar como referencia las affordances de Nuvio/Stremio, con el vídeo como protagonista y controles discretos. El reproductor de directo debe tener ergonomía de IPTV. Esta excepción no autoriza copiar marcas o recursos gráficos ni añadir capacidades de backend.

## Goals / Non-Goals

Objetivos: orientación clara entre tipos de contenido, lectura cómoda, controles previsibles, arte reconocible, densidad útil y navegación móvil completa. Pantallas de acceso, conexión y errores deben ayudar a completar su acción sin decoración que desplace lo importante.

Fuera de alcance: contratos o lógica de negocio, nueva navegación funcional, proveedores o fuentes, búsqueda/recomendaciones adicionales, cambios de metadata, códecs, favoritos o sus claves, nuevos recursos de backend y publicación. No se promete compatibilidad de reproducción adicional por un rediseño.

## Decisions

### Escena y lenguaje visual

Escena: una persona elige qué ver por la noche en el salón, con luz tenue y el contenido a punto de ocupar la pantalla; necesita reconocer títulos, moverse sin esfuerzo y evitar superficies deslumbrantes. Esto justifica mantener el tema oscuro exigido por AGENTS.md y concentrar el contraste en texto y controles.

Usar estrategia restringida: neutros teñidos, un acento para acciones/selección/foco y colores semánticos discretos para estados. Definir tokens OKLCH en un único lugar, sin negro/blanco absolutos, texto con gradiente, cristal decorativo, franjas laterales de acento o paneles anidados. La paleta exacta se documentará después de cargar contexto; puede ajustarse sin cambiar estos requisitos. Las fotografías y carteles reales aportan variedad, sin filtros que impidan reconocerlos.

Una familia sans local o de sistema para controles y datos. Escala fija en rem, títulos de página claramente mayores que cuerpo, cuerpo normal de referencia 1rem, etiquetas compactas legibles y prosa limitada a 75ch. No usar tipografía de exhibición en botones o formularios. Reservar los títulos largos con wrapping o elipsis controlada; el nombre completo debe poder leerse en ficha o mediante nombre accesible.

### Identidad propia frente a Nuvio y Stremio

No copiar logotipos, mascotas, colores de marca, frases ni iconografía distintiva de Nuvio o Stremio. La marca de navegación, acceso y reproductor reutiliza el logo oficial de la aplicación WalacTV existente (`apps/web/public/logo.png`) y su favicon (`apps/web/public/favicon.png`). La referencia de interacción y composición del reproductor VOD se permite expresamente; no se extiende al catálogo ni a la identidad global. La biblioteca de WalacTV se presentará como índice personal y guía de canales: encabezado editorial compacto, búsqueda y filtros directos, y catálogo IPTV del usuario. Evitar el gran banner promocional genérico y los elementos decorativos que hacen que la interfaz parezca una portada intercambiable de streaming. Mantener una cuadrícula de carteles cuando ayude a reconocer películas y series; la diferenciación viene de la identidad, jerarquía y uso de la página, no de ocultar el contenido.

### Estructura por superficie

- Acceso: marca contenida, propósito breve, formulario como acción dominante, etiquetas persistentes y error junto al formulario. Conservar autocompletado y envío actuales.
- Shell: navegación estable y sección activa identificable sin depender solo del color; acceso claro a cuenta/fuente. Cerrar sesión debe llamarse así y no presentarse como selector desplegable si cierra directamente.
- Sin fuente: formulario privado y explicación breve del formato aceptado/cifrado. Estado de recuperación separado de catálogo vacío; conservar reintento y confirmación antes del borrado.
- Directo: logo contenido, nombre legible y acción de reproducir; evitar recortar logos como carteles. VOD: carátulas con proporción estable, información secundaria breve y acceso claro a ficha. Se admite grid de carteles porque representa contenido homogéneo, sin convertir cada elemento en una tarjeta genérica con icono y párrafo.
- Catálogo: título, búsqueda y filtros tienen prioridad sobre cabeceras decorativas. Los filtros siguen el orden búsqueda → idioma → categoría en Directo, Películas y Series, tanto en el DOM y recorrido Tab como en su presentación móvil/escritorio. Conservar valores, handlers y persistencia al reordenarlos. Mantener contadores, idioma aproximado, sincronización, paginación y seguir viendo existentes. No inventar un título destacado o contenido recomendado.
- Mi lista/Ace Stream: mismo vocabulario de controles y estados. Mantener las acciones existentes; conservar información de aplicación externa y restricciones por plataforma sin prometer compatibilidad nueva.
- Fichas/episodios/confirmación: mejorar jerarquía y scroll interno de los diálogos existentes, con acción principal visible, datos/procedencia disponibles y temporadas. No crear más modales para navegación normal. En móvil pueden ocupar casi toda la pantalla y deben permitir alcanzar cierre y acciones sin desbordamiento.
- Reproductor de directo: escena centrada en el canal, indicador «En directo», título, salida al catálogo y controles de reproducción/sonido/fullscreen accesibles. Presentación compacta de IPTV sin mostrar como VOD una duración o progreso de emisión no conocidos. No se inventan parrilla, programa actual, cambio de canal ni timeshift.
- Reproductor de películas/episodios: experiencia inmersiva inspirada en Nuvio/Stremio, con vídeo dominante, título y controles discretos. Conservar reproducción/pausa, sonido, fullscreen, reanudación/progreso y búsqueda cuando el modo actual y una duración fiable lo permitan. Mantener controles nativos en reproducción directa y los controles existentes de conversión; no duplicar dos barras activas. El detalle de temporada/episodios sigue en la ficha, sin añadir navegación entre episodios al player.
- Ambos modos: volver/título/estado legibles, carga, inicio manual, error, reintento y diagnóstico saneado disponibles; conversión solo donde ya existe. Los estados no deben tapar permanentemente controles o reproducir sin acción nueva del usuario. Salir conserva liberación del stream y lease; VOD conserva guardado de progreso.

### Texto útil y capacidades comprobadas

Eliminar eslóganes, subtítulos que repiten el heading, estados decorativos de sesión/dispositivo/cuenta y explicaciones obvias de acciones. Mantener etiquetas persistentes, errores accionables, estados de carga necesarios, privacidad de la fuente, cautela sobre idiomas inferidos, atribución TMDB y compatibilidad/plataformas de Ace Stream. Una acción equivalente de salida puede agruparse en un único control claro.

La inspección de `PlayerComponent` y `PlaybackStateService` confirma un único elemento vídeo, tipo `live|movie|episode`, título y contexto de reproducción, heartbeat, salida/liberación, progreso VOD, fullscreen, sonido y fallback local de conversión VOD. La reproducción directa usa controles nativos; conversión tiene controles propios y barra solo con duración fiable. El player no recibe catálogo de canales, EPG, pistas seleccionables, fuentes alternativas ni lista de episodios. Esta tarea modifica presentación por tipo y reutiliza capacidades existentes; no incorpora selectors vacíos ni botones que prometan esas funciones. La disponibilidad de controles nativos depende del navegador.

### Estados, accesibilidad y respuesta

Definir estados default/hover/focus/active/disabled/loading/error según el control. Foco visible de al menos 2px con contraste 3:1 frente a superficie adyacente; texto normal 4.5:1 y texto grande 3:1. Controles principales táctiles de al menos 44×44 CSS px; iconos con nombre accesible, labels persistentes, landmarks y headings coherentes. No usar solo color para seleccionar o fallar.

Diálogos existentes: al abrir mover foco al diálogo/primer control útil, mantener Tab dentro, Escape cierra cuando no hay operación que lo impida y devolver foco al disparador si sigue presente. La confirmación destructiva distingue cancelar/eliminar. Estados de progreso no provocan anuncios repetitivos; errores accionables usan el canal accesible correspondiente.

Transiciones breves (150–250ms) de opacity/transform/color, sin animar layout ni secuencias de entrada. `prefers-reduced-motion: reduce` elimina movimiento y shimmer no esenciales. Los placeholders reservan tamaño y no reemplazan toda la pantalla durante filtros si ya hay un patrón de carga utilizable.

En móvil la navegación se reorganiza, filtros se apilan o distribuyen en filas y acciones no dependen de hover. No habrá scroll horizontal del documento a 320 CSS px ni pérdida de controles a zoom 200%. No se exige orientación ni se bloquea landscape del reproductor. Comprobar esquinas seguras cuando haya controles pegados al borde.

### Compatibilidad y rendimiento

Modificar templates/estilos y, si es necesario, estructura de componentes conservando enlaces, handlers, servicios, modelos, rutas y contratos. Preservar filtros por sección/página, restauración al volver y eliminación al cambiar fuente/sesión. No refactorizar lógica de reproducción para estilizarla.

Mantener lazy routes y arte lazy con dimensiones reservadas/fallback; no añadir descargas de vídeo, solicitudes de metadata, fuentes remotas ni dependencias visuales pesadas. Mantener la paginación; no renderizar el catálogo completo. Documentar tamaño de build antes/después y respetar budgets Angular. Ningún secreto o recurso privado irá en fixtures, capturas, assets o contexto Impeccable.

## Migration Plan

No hay migración de datos, variables nuevas ni cambios Compose. Las rutas y contratos permiten volver a la imagen web previa sin modificar PostgreSQL o la API. Mantener el arranque local documentado y no ejecutar migradores de nuevo por este cambio visual. El coordinador reconstruye solo la web local si necesita mostrar el resultado. Push, Auto Deploy y Dokploy requieren su autorización/destino propios.

## Verification

Registrar capturas antes/después con datos saneados de acceso, sin fuente, directo, películas, series/ficha/episodios, favoritos, Ace Stream y reproductor, a 1440×900 y 390×844; comprobar además documento a 320px y 768px, zoom 200%, teclado y movimiento reducido. Reproducir fallos/ausencias con estados o recursos simulados, sin abrir simultáneamente streams reales. Una captura del reproductor puede usar vídeo local/simulado. Revisar contraste calculado de tokens, foco visible, labels, todos los destinos móviles y errores/carga/vacío. Compilar Angular y comparar budgets. Hacer smoke de búsqueda+idioma+categoría→ficha/player→volver, fuente/sesión y controles existentes sin ampliar backend. Comprobar por separado directo, película, episodio y conversión VOD; verificar orden visual/DOM/Tab de filtros, ausencia de microcopy redundante y ausencia de controles de funciones no disponibles. Registrar límites de la comprobación en vez de afirmar reproducción exhaustiva.

## Open Questions

La referencia Nuvio/Stremio se limita al reproductor VOD por la instrucción posterior del usuario. El directo usa ergonomía IPTV y el catálogo conserva identidad propia. No quedan decisiones de negocio abiertas ni se añaden capacidades de backend.
