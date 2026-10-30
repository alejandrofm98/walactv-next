# Proposal

## Why

WalacTV reúne directo, películas, series y biblioteca privada, pero sus pantallas necesitan un lenguaje visual coherente que facilite encontrar contenido y ver una emisión cómodamente. El usuario ha solicitado mejorar toda la web con Impeccable y ha confirmado una interfaz de producto con sensación de cine en casa inmersiva.

## What Changes

- Definir contexto de producto y tokens visuales compartidos: superficies oscuras legibles, tipografía, espaciado, controles, estados y movimiento.
- Diferenciar WalacTV de Nuvio y Stremio con una identidad propia de filmoteca/guía personal, evitando copiar sus marcas y convenciones reconocibles.
- Rediseñar acceso, navegación, conexión de fuente, directo, películas, series, favoritos, Ace Stream, fichas, confirmaciones y reproductor.
- Dar protagonismo al contenido real con tratamientos distintos para logos de directo y carátulas VOD, sin inventar imágenes ni metadatos.
- Adaptar estructura y controles a escritorio y móvil con foco, teclado, contraste y movimiento reducido verificables.
- Conservar contratos, acciones y restricciones actuales, incluidos filtros al regresar y limpieza de reproducciones.

## Capabilities

### New Capabilities

- `immersive-web-experience`: lenguaje visual, jerarquía, componentes, estados y adaptación de todas las superficies existentes.

### Modified Capabilities

Ninguna especificación consolidada existe aún en `openspec/specs`. Este cambio complementa los artefactos de navegación de catálogo, persistencia de filtros, metadata y reproducción existentes, sin sustituir sus contratos ni políticas.

## Impact

Frontend Angular, estilos y recursos locales, contexto Impeccable y documentación de verificación visual. No cambia API Go, endpoints, autenticación, consultas, integración TMDB, PostgreSQL, migraciones, Compose, variables, imagen API ni límites de conexiones/conversión. No añade reproducción automática, carruseles con consultas nuevas ni capacidades de biblioteca. Publicación y despliegue quedan fuera del encargo.
