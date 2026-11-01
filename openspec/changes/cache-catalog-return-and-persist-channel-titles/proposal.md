# Proposal

## Why

Al volver del reproductor, la web conserva los filtros pero consulta de nuevo el catálogo y retrasa la recuperación de resultados que aún tiene en memoria. La limpieza actual de `###` se aplica en algunas vistas; el usuario solicita que el nombre mostrado limpio quede persistido y sea uniforme en toda la aplicación.

## What Changes

- Restaurar el último resultado y sus facetas al volver desde reproducción dentro de la misma sesión, fuente y consulta, sin repetir peticiones de items ni facetas.
- Invalidar esa copia al cambiar consulta, fuente o sesión, al actualizar explícitamente el catálogo o cuando no se completó una carga válida.
- Persistir un nombre mostrado de canal que omita grupos de dos o más `#` y normalice espacios, conservando el título original del proveedor y etiquetas útiles de idioma, calidad y versión.
- Aplicar la limpieza a las filas ya guardadas mediante una migración aditiva compatible con la API anterior.
- Mantener identificadores, claves de reproducción y permisos existentes. La reutilización y el significado del parámetro de URL `o` quedan fuera de este cambio.

## Capabilities

### New Capabilities

- `catalog-navigation`: restauración inmediata de la vista privada del catálogo. No existe aún especificación principal archivada; esta delta sustituye el escenario de reconsulta de `preserve-catalog-filters-and-clean-channel-titles` al consolidar las propuestas pendientes.
- `provider-channel-titles`: conservación del título original y persistencia de un nombre mostrado normalizado para canales.

### Modified Capabilities

Ninguna capacidad está archivada actualmente en `openspec/specs`.

## Impact

Angular: estado de navegación del catálogo, facetas y resultados; enlaces derivados del resultado restaurado. Go/PostgreSQL: lectura, búsqueda y presentación de nombres del catálogo; migración aditiva sobre `source_catalog_entries`. Se reutilizan los servicios Compose `migrate` y `api`, sin variables, dependencias, puertos ni servicios nuevos. Las comprobaciones se realizan en local o con proveedores simulados; publicar o tocar Dokploy requiere el alcance de autorización correspondiente.
