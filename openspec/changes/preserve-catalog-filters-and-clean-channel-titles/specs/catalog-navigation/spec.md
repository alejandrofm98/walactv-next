## ADDED Requirements

### Requirement: Conservar filtros al volver de reproducción
La aplicación SHALL conservar sección, búsqueda, categoría, idioma aproximado y página seleccionados cuando el usuario abre un canal, película o episodio y vuelve al catálogo, siempre que siga en la misma sesión y fuente IPTV.

#### Scenario: Regreso al catálogo
- **WHEN** el usuario reproduce un elemento con filtros aplicados y vuelve al catálogo
- **THEN** se restauran los mismos filtros y la página, y se vuelve a consultar el catálogo con esos valores

#### Scenario: Cambia la fuente o termina la sesión
- **WHEN** se reemplaza/elimina la fuente IPTV, se cierra la sesión o expira
- **THEN** se limpian los filtros y resultados privados antes de presentar otro catálogo

### Requirement: Presentar nombres limpios de canales
La aplicación SHALL omitir grupos repetidos de `#` en el nombre visible de un canal en vivo, sin modificar títulos de películas, series, episodios, datos del proveedor ni claves de reproducción.

#### Scenario: Canal con marcadores de proveedor
- **WHEN** un canal en vivo tiene `###` en su título
- **THEN** tarjetas, favoritos y reproductor muestran el título sin esos marcadores y conservan el resto del texto

### Requirement: Mensajes de fallo de reproducción precisos
La aplicación SHALL evitar atribuir al formato o códec un fallo que también puede deberse a que el proveedor no entregó el flujo.

#### Scenario: El recurso de vídeo falla al cargar
- **WHEN** el elemento de vídeo comunica un error de carga
- **THEN** la interfaz explica que el flujo puede faltar o ser incompatible y ofrece probar otra versión, sin afirmar una causa que no se conoce
