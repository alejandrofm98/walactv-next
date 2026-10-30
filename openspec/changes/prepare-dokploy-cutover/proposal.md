# Proposal

## Why

WalacTV Next funciona localmente y necesita un destino productivo verificable para sustituir la web anterior. El ecosistema anterior también sirve a Android, por lo que el corte debe conservar sus dependencias y permitir recuperar el dominio si falla la nueva versión.

## What Changes

- Preparar la publicación del repositorio público nuevo, la configuración de Compose y el contexto de Dokploy sin exponer secretos.
- Documentar un proyecto/servicio separados, PostgreSQL externa propia y despliegue inicial sin mover el dominio final ya asignado a la web anterior.
- Definir promoción al dominio antiguo, comprobación de versión/salud y recuperación hacia el servicio web anterior, conservándolo durante una ventana de recuperación acordada.
- Registrar candidatos de retirada por consumidores y datos: la API IPTV y su scraper siguen siendo dependencias de Android según el contexto local; los servicios de desarrollo y las bases requieren comprobar uso actual.
- **BREAKING**: el corte de la web introduce la autenticación y base propias de la aplicación nueva. No migra automáticamente usuarios, sesiones, listas ni biblioteca del sistema anterior.

## Capabilities

### New Capabilities

- `production-web-release`: publicación y sustitución verificables de la web con aislamiento de datos, configuración segura y recuperación del dominio.

### Modified Capabilities

Ninguna. Se conservan los contratos de la aplicación nueva y los servicios anteriores.

## Impact

Afecta a documentación de despliegue, preparación Git/GitHub, configuración productiva de Compose y recursos/rutas de Dokploy cuando exista destino confirmado. La API Go, web Angular y migraciones 001–004 ya existen; no se añade esquema ni migración en este cambio. No se importa la base legacy ni se borra ningún servicio, volumen o base. La web y servicios legacy se revalidaron en el panel el 2026-10-30; el usuario eligió como destino final el hostname de la web actual, pero el nuevo servicio y recursos productivos todavía deben configurarse.
