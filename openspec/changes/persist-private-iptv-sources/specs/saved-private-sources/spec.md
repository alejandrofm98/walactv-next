# Spec Delta

## Purpose

Conservar fuentes IPTV y catálogo consultado por usuario para recuperar la configuración tras iniciar sesión o reiniciar sin almacenar credenciales del proveedor en claro.

## ADDED Requirements

### Requirement: Fuente guardada cifrada y privada
El sistema SHALL guardar la fuente, URL original y credenciales cifradas autenticadamente antes de escribir PostgreSQL, con clave externa estable e identidad de propietario y fuente vinculadas al cifrado. SHALL no devolver secretos ni incluirlos en logs, fixtures o catálogo persistido.

#### Scenario: Inspección de persistencia
- **WHEN** se guarda una fuente válida
- **THEN** el registro mantiene ID/propietario/metadatos y ciphertext sin URL privada, usuario ni contraseña en claro

#### Scenario: Datos o clave incorrectos
- **WHEN** se usa clave incorrecta, falta clave o se altera ciphertext/propietario/sourceID
- **THEN** se deniega el descifrado sin borrar, sustituir ni regenerar silenciosamente los datos

### Requirement: Restauración y ciclo de vida
El sistema SHALL recuperar únicamente fuentes propias tras login/reinicio con el mismo ID estable. Logout SHALL revocar medios y RAM sin borrar persistencia; eliminar fuente SHALL borrar fuente/catálogo persistentes y evitar su reaparición.

#### Scenario: Reinicio y logout
- **WHEN** A guarda una fuente, cierra sesión y vuelve a entrar tras reiniciar API
- **THEN** recupera la fuente propia sin reintroducir credenciales y sin restaurar streams antiguos

#### Scenario: Acceso cruzado y borrado
- **WHEN** B intenta listar o hidratar una fuente de A
- **THEN** no puede acceder a ella
- **WHEN** A elimina la fuente y reinicia
- **THEN** ni fuente ni catálogo borrados reaparecen

### Requirement: Catálogo persistente saneado
El sistema SHALL guardar bajo demanda categorías/títulos/episodios con IDs, tipos, extensiones y contentKeys validados, excluyendo URLs de stream y direct_source. SHALL reutilizarlo durante 60 segundos y refrescar tras vencer sin servir datos obsoletos como actuales.

#### Scenario: Caché y fallo del proveedor
- **WHEN** se vuelve a consultar el mismo catálogo propio antes de 60 segundos
- **THEN** puede responder desde persistencia sin descargar todo el catálogo
- **WHEN** ha caducado y el proveedor no responde
- **THEN** devuelve error contextualizado sin fallback stale ni borrar el catálogo guardado

### Requirement: Revalidación antes de reproducir
El sistema SHALL revalidar estado de cuenta y límite de conexiones antes de reproducir una fuente hidratada o con validación anterior a 60 segundos. La configuración guardada SHALL no prolongar autenticación ni restaurar leases revocados.

#### Scenario: Cuenta guardada deshabilitada
- **WHEN** el proveedor ha deshabilitado la cuenta tras guardar/reiniciar
- **THEN** no se inicia reproducción aunque exista catálogo persistido

### Requirement: Migración y recuperación controladas
El sistema SHALL aplicar migración aditiva versionada bajo lock y conservar compatibilidad del esquema previo. La recuperación SHALL requerir backup y clave original, sin interpretar rollback de imagen como reversión de PostgreSQL.

#### Scenario: Actualización con datos
- **WHEN** se migra una base local con usuarios/favoritos/progreso existentes
- **THEN** se conservan esos datos y se aplica cada versión una sola vez
