# Spec Delta

## Purpose

Recuperar con rapidez la vista privada del catálogo al volver de reproducción, conservando una copia coherente de los resultados y los filtros de la sesión actual.

## ADDED Requirements

### Requirement: Restauración inmediata del resultado de catálogo
La web SHALL reutilizar el último resultado completado y sus facetas al volver de reproducción cuando usuario, sesión, fuente, sección, categoría, idioma, búsqueda y página coincidan. SHALL restaurar filtros, total y acciones sin volver a pedir items, categorías ni idiomas al servidor.

#### Scenario: Regreso a una búsqueda completada
- **WHEN** el usuario abre un canal desde una búsqueda completada y vuelve con Atrás o Volver al catálogo
- **THEN** ve los mismos resultados, total, filtros y página sin nueva petición de búsqueda ni facetas
- **AND** puede abrir otro resultado porque sus enlaces están disponibles

#### Scenario: Resultado vacío válido
- **WHEN** vuelve a una consulta completada sin coincidencias
- **THEN** se restaura el estado vacío y su total cero sin repetir la consulta

#### Scenario: Consulta sin resultado completado
- **WHEN** no existe copia correspondiente a los filtros y página restaurados, o la petición anterior falló o se canceló
- **THEN** se consulta el catálogo con los valores restaurados y se muestran los estados habituales de carga o error

### Requirement: Invalidez de la copia de navegación
La web SHALL descartar resultados privados reutilizables al cambiar o eliminar fuente, cerrar o expirar sesión o iniciar actualización explícita del catálogo. Una consulta distinta SHALL requerir su propio resultado completado. La copia SHALL permanecer en memoria de la sesión de la pestaña.

#### Scenario: Cambia fuente o usuario
- **WHEN** la sesión termina o se reemplaza o elimina la fuente
- **THEN** los resultados, facetas y filtros anteriores no se presentan como datos de la nueva sesión o fuente

#### Scenario: Cambio de búsqueda o actualización manual
- **WHEN** el usuario cambia búsqueda, sección, filtros o página, o solicita Actualizar lista
- **THEN** la copia anterior no evita la consulta necesaria para ese cambio
- **AND** al completar la nueva carga se puede reutilizar su resultado al volver del reproductor

#### Scenario: Recarga completa
- **WHEN** se recarga la página y ya no existe la copia en memoria
- **THEN** el catálogo se consulta normalmente sin depender de resultados privados guardados en almacenamiento persistente del navegador
