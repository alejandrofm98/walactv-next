# Proposal

## Why

El usuario ha pedido conservar lista, usuario, contraseña y catálogo para no introducir la fuente de nuevo. La persistencia debe mantener el aislamiento privado y evitar almacenar secretos del proveedor en claro.

## What Changes

- Guardar una fuente por usuario en PostgreSQL con ID estable y JSON de credenciales cifrado en Go; restaurarla al iniciar sesión/reiniciar sin restaurar streams anteriores.
- Guardar bajo demanda catálogo saneado de categorías, títulos y episodios; excluir URLs proveedor/direct_source y secretos.
- Mantener TTL 60 s del catálogo sin fallback obsoleto y revalidar cuenta/límite antes de reproducir una fuente hidratada o validada hace más de 60 s.
- Cerrar sesión revoca RAM/streams pero conserva la fuente; «Eliminar fuente» borra su registro y catálogo.
- Añadir clave externa estable, migración aditiva 002 y pruebas de reinicio, aislamiento, cifrado y recuperación.

## Capabilities

### New Capabilities

- `saved-private-sources`: fuente cifrada y catálogo saneado persistentes, recuperación y ciclo de vida privado.

### Modified Capabilities

Ninguna: no hay specs principales archivadas. Este cambio sustituye la política efímera del cambio build-greenfield-walactv sin reescribir sus artefactos históricos.

## Impact

API Go/store/migrador y UI de fuentes, configuración Compose y setup local, contrato API y guía operativa. PostgreSQL existente del proyecto recibe tablas nuevas mediante `walactv migrate`, una vez antes del API y con lock de versiones; sin tocar sistemas anteriores.

`IPTV_ENCRYPTION_KEY` base64 de 32 bytes, externo a PostgreSQL/Git, pasa únicamente al servidor. Recuperar datos exige backup y clave original; una imagen anterior no revierte esquema ni recupera secretos con otra clave. Implementación local autorizada; sin push ni despliegue productivo.
