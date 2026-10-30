# Design

## Context

El usuario ha pedido guardar lista, URL y credenciales para evitar reentrada. El núcleo existente tiene fuentes/catálogos en `privateState` RAM y PostgreSQL para identidad/biblioteca; sus tipos CatalogItem/Episode contienen URL interna excluida del JSON. La nueva persistencia debe proyectar campos seguros, no serializar estructuras internas ciegamente. El migrador actual conoce 001; necesita recorrer versiones ordenadas bajo el mismo lock.

Ver `docs/api-contract.md`. Este cambio está autorizado para implementación local y sustituye la política RAM-only del MVP sin modificar artefactos históricos ni proyectos anteriores.

## Goals / Non-Goals

**Goals:** recuperar automáticamente fuente y catálogo consultado propios tras login/reinicio, secretos cifrados y borrado explícito fiable.

**Non-Goals:** descargar toda la lista al guardar, devolver/editar secretos en UI, sesiones/streams persistentes, rotación automática de claves, multirréplica o importación legacy.

## Decisions

### Fuente cifrada por propietario

Migración aditiva `002` crea fuente privada con ID estable, owner FK, accountKey/label, límite declarado, versión criptográfica y ciphertext; un registro por propietario. JSON secreto incluye URL original, base normalizada, usuario y contraseña. Cifrar en Go con AES-256-GCM, nonce aleatorio nuevo por escritura, AAD canónico y no ambiguo con versión+ownerID+sourceID; almacenar nonce/ciphertext como bytea o envelope versionado. Clave `IPTV_ENCRYPTION_KEY`: base64 de exactamente 32 bytes, estable, del entorno/gestor de secretos externo a DB/Git. Nunca derivarla de credenciales proveedor ni confundirla con PROVIDER_IDENTITY_KEY.

Validar clave al iniciar API; ausencia/formato inválido falla cerrado. Ciphertext alterado/clave errónea deniega descifrado con error seguro sin borrar/reescribir ni generar una nueva clave. No logs SQL de parámetros/JSON secretos, errores de AEAD genéricos y sin URLs. Metadata pública no contiene base, username, password ni URL original. sourceID persistente y accountKey estable conservan biblioteca; reemplazar fuente solo después de validación/protección transaccional, sin perder una existente por un alta inválida.

### Restauración, sesión y borrado

GET /sources lista metadata guardada del owner autenticado; no necesita proveedor accesible para listar. La API hidrata descifrado por owner al necesitar catálogo/playback y reconstruye estado RAM con límites de la sesión vigente, nunca la sesión guardada anterior. Solo credentials se recuperan; playbacks/leases nunca se restauran. Logout elimina RAM/medios de usuario, no fila persistente. TTL RAM 30 min permite volver a hidratar; autenticación absoluta/CSRF siguen iguales.

DELETE autentica owner, cancela streams y borra fuente/catálogo persistente en transacción; FK CASCADE o borrado equivalente. Evitar resurrección de un catálogo tras DELETE concurrente con fetch: escrituras exigen fuente viva/owner y respetan FK/transacción o generación comprobada; limpiar RAM y trabajo en vuelo. Una fuente no vuelve a aparecer tras reinicio. UI muestra guardada y diferencia «Cerrar sesión»/«Eliminar fuente» sin checkbox: el usuario ya autorizó persistencia.

### Catálogo seguro y frescura

Tabla JSONB por sourceID+owner+tipo de consulta+parámetros normalizados (categorías por tipo, items por categoría, episodios por serie), con fetchedAt/expiresAt y datos de esquema explícito. Permitir solo IDs/nombres/títulos/tipo/extensión/season/episode/categoryID/seriesID/contentKey necesarios para navegación. No guardar URL interna, `direct_source`, stream URL, imagen/query con secretos ni respuesta raw Xtream. Al reproducir reconstruir URL mediante credenciales descifradas y metadata validada, no confiar en JSONB como URL autorizada. Usar consultas owner-scoped y claves RAM también por owner/accountKey.

Guardar únicamente consultas solicitadas, conservando categoría/serie/contentKey. TTL común RAM/PG de 60 s desde fetchedAt; reiniciar no renueva artificialmente vigencia. Dentro del TTL reutilizar; fuera consultar proveedor y sustituir snapshot saneado atómicamente. **Sin fallback stale**: fallo proveedor devuelve 502/504 contextualizado, conserva snapshot pero no lo presenta como fresco. Sin nuevo campo stale ni cambio de forma categorías/items/episodes.

Antes de POST playback revalidar cuenta/max_connections al hidratar y cuando última validación tenga más de 60 s; si inactiva o falla, no abrir upstream ni lease nuevo. Catálogo en caché no acredita autorización vigente. Conservar arbitraje max_connections por cuenta y condición single-process/single-replica: PostgreSQL de fuentes no hace compartidos los leases RAM.

### Contrato y configuración

Endpoints actuales iguales. Source añade `saved:true`; id persistente, expiresAt solo de copia RAM y no fecha de borrado PG. Logout conserva guardado; DELETE elimina definitivamente. Ninguna respuesta devuelve secretos. Infra añade clave únicamente API al Compose y script local; generar una vez para instalación nueva y preservar .env existente. Si hay fuente cifrada y falta clave, operador debe recuperar clave original, no generar otra silenciosamente.

Documentar nombre de variable y recuperación en deployment/README sin valor. PostgreSQL de producción sigue externo y sin destino configurado. La implementación es local; nada autoriza push o migrar otras bases.

## Risks / Trade-offs

- [Pérdida de clave] → backup DB no basta; conservar clave externa de forma recuperable, ensayar restore local y no prometer descifrado sin ella.
- [Catálogo caducado/proveedor caído] → error explícito sin stale; se preserva último snapshot para próximo refresco válido.
- [Contraseña proveedor cambiada] → fuente guardada puede fallar; revalidar y permitir eliminar/reintroducir sin borrar datos automáticamente.
- [Persistencia y escala] → leases y medios siguen RAM/una instancia; no declarar HA por guardar fuentes en PG.

## Migration Plan

1. Añadir 002 sin modificar 001; migrador enumera todos los SQL ordenados, aplica pendientes una vez bajo advisory lock y transacción por versión. Verificar DB existente con usuarios/favoritos/progreso y migradores concurrentes.
2. Configurar clave estable privada y ejecutar `walactv migrate` antes del API local. Una fuente RAM previa no se guarda retroactivamente sin POST válido; primera alta posterior queda cifrada.
3. Probar guardar/cerrar sesión/reiniciar/listar/hidratar/reproducir/eliminar con fixtures; verificar DB sin secretos plaintext y aislamiento A/B. La prueba real la coordina el padre sin mostrar credenciales.
4. Rollback a imagen anterior puede ignorar tablas nuevas; no ejecutar DROP/down automático. Esa imagen vuelve al comportamiento efímero y no puede prometer mostrar fuentes guardadas. Volver a imagen compatible y clave original recupera acceso; restauración exige backup + clave. No tocar tablas previas ni sistemas anteriores.
