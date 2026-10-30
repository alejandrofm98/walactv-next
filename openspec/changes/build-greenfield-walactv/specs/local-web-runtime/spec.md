# Spec Delta

## Purpose

Entregar una aplicación local nueva, navegable y verificable que arranque con datos independientes y no requiera compatibilidad ni servicios del sistema anterior.

## ADDED Requirements

### Requirement: Interfaz y ejecución independientes
La aplicación SHALL ofrecer interfaz oscura en español, login, fuente, catálogos y reproductor bajo demanda con estados de carga/error. SHALL funcionar sin API Python, servir web/API por mismo origen y no descargar HLS antes de abrir reproducción.

#### Scenario: Flujo local completo
- **WHEN** se inicia sesión, registra fuente, selecciona un título y vuelve al catálogo
- **THEN** puede reproducir y continuar navegando con contexto de selección, sin peticiones a la API anterior

### Requirement: AceStream local voluntario
La aplicación SHALL validar IDs de 40 hex y abrir protocolo en Windows/Linux o intent en Android mediante gesto explícito, sin sondear motores localhost ni crear P2P en backend. SHALL explicar iOS no compatible con AceStream sin bloquear IPTV compatible.

#### Scenario: ID inválido
- **WHEN** se introduce una URL ajena o un ID incompleto
- **THEN** no se ejecuta navegación externa y se muestra error

#### Scenario: Apertura válida
- **WHEN** se pulsa abrir con ID válido
- **THEN** se construye protocolo/intent validado y se ofrecen instrucciones de aplicación instalada

### Requirement: PostgreSQL y migración independientes
El sistema SHALL usar PostgreSQL nuevo local, separado de proyectos anteriores, y migraciones SQL versionadas con bloqueo único antes del servidor. Compose base SHALL aceptar PostgreSQL externo y el override local SHALL usar volumen propio; un rollback de imagen SHALL no revertir automáticamente el esquema.

#### Scenario: Primera instalación
- **WHEN** se inicia desde una base local vacía y se ejecuta migración/bootstrap
- **THEN** se crea esquema y administrador nuevos sin leer ni modificar datos anteriores

#### Scenario: Ejecución concurrente
- **WHEN** dos migradores intentan aplicar la misma versión
- **THEN** solo uno la aplica y queda versión consistente o error explícito que impide declarar listo el servicio

### Requirement: Alcance local verificable
La entrega local SHALL acreditar contratos, build, pruebas deterministas y navegador para el núcleo. SHALL distinguir funciones futuras y preparación productiva de lo implementado, sin exigir paridad legacy, importación o Android para completar el MVP local.

#### Scenario: Cierre del MVP
- **WHEN** funcionan los flujos del núcleo y sus comprobaciones
- **THEN** se informa su resultado local y se dejan explícitos pendientes de producción, sin afirmar publicación ni estabilidad prolongada no comprobada
