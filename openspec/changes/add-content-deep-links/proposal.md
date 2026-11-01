# Proposal

## Why

La primera implementación local introduce enlaces privados recuperables, pero sus rutas técnicas en inglés exponen el identificador largo de fuente y no permiten reconocer el título. El usuario necesita URLs concisas en español que conserven la identidad inequívoca, la autenticación y la navegación ya implementadas.

## What Changes

- Sustituir las rutas técnicas por `/directo`, `/peliculas` y `/series`, con IDs de item y slugs legibles; usar `/ver` y `/episodios` para reproducción.
- Resolver la única fuente activa desde la sesión; retirar su identificador largo del path y conservar una referencia compacta de origen para rechazar enlaces de fuentes sustituidas.
- Tratar el slug como decoración canónica, nunca como identidad ni autorización; mantener IDs y validación de pertenencia de episodios.
- Resolver contenido desde la URL con sesión y autorización, sin depender de tarjetas cargadas ni de un playback previo en memoria.
- Integrar historial, cierre de fichas y salida del reproductor conservando filtros y limpieza de recursos existentes.
- Recuperar el destino interno después de iniciar sesión y distinguir contenido inexistente, fuente no disponible y fallos transitorios.
- Añadir una consulta puntual autenticada de catálogo por ID reutilizando la persistencia actual.

## Capabilities

### New Capabilities

- `content-deep-links`: navegación privada recuperable a contenido y reproducción desde URLs estables.

### Modified Capabilities

Ninguna. Este cambio complementa `catalog-navigation`, `vod-catalog-navigation`, `private-session-library` e `immersive-web-experience`, conservando sus requisitos de filtros, privacidad, accesibilidad y playback.

## Impact

Afecta al router Angular, guards/login, biblioteca/fichas, estado y ciclo de reproducción y consulta puntual del API Go. Nginx ya entrega `index.html` para rutas de la SPA; conservar su configuración, `/api/` y assets sin añadir aceptación manual. No requiere cambios de Compose, variables, servicios ni esquema PostgreSQL; no hay migraciones que ejecutar. La consulta API es aditiva. Las rutas técnicas anteriores se redirigen tras autorizar su fuente exacta, mediante reemplazo de historial. `/player` conserva compatibilidad limitada al estado de sesión; no identifica contenido por sí sola. Una imagen anterior seguirá funcionando con la misma base, aunque no resolverá las nuevas URLs.

Fuera de alcance: enlaces públicos o entre cuentas/fuentes distintas, SEO, búsqueda por slug, migración legacy, zapping, nuevas capacidades de reproducción, rediseño visual, publicación y despliegue.
