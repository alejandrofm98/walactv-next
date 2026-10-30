# Spec Delta

## Purpose

Facilitar descubrimiento y reproducción de películas y series guardadas, mostrando metadatos disponibles con procedencia y privacidad claras.

## ADDED Requirements

### Requirement: Navegación diferenciada de películas y series
La web SHALL ofrecer secciones Directos/Películas/Series con búsqueda, categorías e idioma por tipo, datos del catálogo persistido y estados de carga/error/vacío. SHALL permitir ver ficha y reproducir películas o seleccionar temporadas/episodios sin adivinar extensiones.

#### Scenario: Película y serie disponibles
- **WHEN** se abre una película o serie guardada
- **THEN** muestra título, carátula si existe y datos proveedor disponibles; película inicia su recurso validado y serie ofrece episodios por temporada antes de reproducir

#### Scenario: Ficha incompleta
- **WHEN** faltan sinopsis, reparto o valoración
- **THEN** no se inventan datos y la acción de reproducción sigue disponible

### Requirement: Procedencia y privacidad de enriquecimiento
El sistema SHALL conservar IPTV como origen de disponibilidad/reproducción y no enviar títulos privados automáticamente a Cinemeta u otros servicios. La investigación SHALL documentar compatibilidad IMDb, API oficial, emparejamientos ambiguos, idioma y límites; una integración futura SHALL exigir identificación verificada o selección explícita y conservar datos proveedor si falta coincidencia.

#### Scenario: Título ambiguo o catálogo sin metadatos externos
- **WHEN** un título tiene múltiples posibles coincidencias o Cinemeta no está integrado/disponible
- **THEN** se mantienen datos proveedor y reproducción sin asociar silenciosamente una película o serie distinta
