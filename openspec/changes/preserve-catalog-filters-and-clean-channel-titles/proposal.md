# Conservar filtros del catálogo al reproducir y limpiar títulos de canales

## Motivo

Al volver del reproductor se recrea la vista del catálogo. La recuperación de la fuente interpreta esa recreación como un cambio de fuente y borra filtros que el usuario había elegido. Algunos nombres de canales también contienen marcadores `###` del proveedor que ocupan espacio sin aportar información.

## Alcance

- Conservar sección, búsqueda, categoría, idioma y página al navegar al reproductor y volver.
- Limpiar el estado del catálogo al cambiar/eliminar la fuente o cerrar/expirar la sesión.
- Ocultar marcadores `###` repetidos solo al presentar nombres de canales en vivo, incluidos favoritos y el reproductor.
- Usar un error de reproducción que no atribuya falsamente al códec los fallos de enlace/proveedor.
- Mantener intactos los títulos originales enviados por el proveedor, las claves de reproducción y la búsqueda.

## Fuera de alcance

- No cambiar filtros entre secciones distintas.
- No modificar películas, series ni títulos de episodios.
- No alterar catálogo, API, datos persistidos ni enlaces de reproducción.
