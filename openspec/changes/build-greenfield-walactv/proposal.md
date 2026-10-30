# Proposal

## Why

El usuario ha autorizado empezar una aplicación nueva desde cero y resolver la reutilización más adelante. La primera entrega local permite validar un núcleo independiente Go/Angular sin bloquearse por migraciones ni compatibilidad del producto anterior.

## What Changes

- Crear API Go y UI Angular standalone nuevas sin dependencia de la API Python.
- Proveer acceso cerrado por administrador, sesión por cookie, PostgreSQL local independiente y biblioteca personal de favoritos/progreso.
- Añadir fuente IPTV privada efímera por usuario, catálogo de directo/películas/series/episodios y reproducción HTTPS de HLS/archivos con control de acceso y Range.
- Crear interfaz oscura con catálogo/reproductor bajo demanda y apertura AceStream local mediante gesto.
- Añadir Compose web/API con PostgreSQL externo por defecto y override PostgreSQL exclusivamente local; migración SQL única antes de arrancar.
- Verificar integración, contratos y navegador local; separar el resultado local de requisitos futuros de producción.

## Capabilities

### New Capabilities

- `private-session-library`: acceso administrativo, sesión y biblioteca personal persistente.
- `private-iptv-playback`: fuente efímera, catálogo aislado y transporte de medios seguro y cancelable.
- `local-web-runtime`: UI nueva, AceStream y ejecución local independiente con migraciones controladas.

### Modified Capabilities

Ninguna; el proyecto nuevo no tiene especificaciones principales.

## Impact

Código nuevo en `apps/api` y `apps/web`; contrato compartido en `docs/api-contract.md`. Sin cambios/importación de datos o clientes anteriores. Agenda/replays/Cast, Android, importación legacy y Torrentio habilitado son alcances futuros y no bloquean este MVP. Tampoco se exige compatibilidad universal de códecs ni conversión FFmpeg para el núcleo.

PostgreSQL nuevo con migraciones versionadas; producción será recurso externo separado de Compose. La implementación y pruebas locales están autorizadas; push/Dokploy/BD productiva no. No hay remoto ni destino de despliegue configurado.
