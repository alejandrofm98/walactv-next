# Tasks

## 1. Cifrado y esquema

- [x] 1.1 Añadir migración aditiva 002 de fuente/catálogo con ownership y borrado en cascada, y migrador de todas las versiones bajo lock; verificar DB con 001/datos existentes, repetición y migradores concurrentes sin pérdida previa.
- [x] 1.2 Añadir AES-256-GCM/nonce aleatorio/AAD versión-owner-source y validación de IPTV_ENCRYPTION_KEY; verificar roundtrip, ciphertext alterado, owner/source cambiado y clave incorrecta sin borrado ni secretos plaintext/logs.

## 2. Persistencia API

- [x] 2.1 Guardar fuente con ID estable, recuperar por owner e hidratar RAM tras TTL/reinicio; verificar logout conserva PG, reinicio recupera metadata/credenciales sin streams y B no accede a fuente A.
- [x] 2.2 Persistir proyección saneada del catálogo bajo demanda y TTL60s RAM/PG; verificar ausencia de URL/direct_source/password, snapshot vigente tras reinicio y error sin stale al caducar/proveedor caído.
- [x] 2.3 Revalidar cuenta/max_connections antes de playback hidratado o validación mayor de 60 s, y borrar fuente/catálogo/RAM en DELETE; verificar cuenta deshabilitada, borrado que no reaparece y carrera fetch/delete sin resurrección con go test -race.

## 3. Interfaz y operación

- [x] 3.1 Mostrar fuente guardada y recuperarla al login/reload sin pedir URL otra vez; verificar saved:true, expiración RAM sin mensaje de borrado y diferencia logout/eliminar mediante pruebas frontend y navegador.
- [x] 3.2 Añadir clave API-only al entorno/Compose/setup y documentar backup+clave/rollback aditivo; verificar setup preserva clave existente, config --quiet y fallo cerrado por clave ausente/incorrecta sin generar sustituta sobre datos existentes.

## 4. Integración local

- [x] 4.1 Ejecutar migración local y flujo guardar→logout→restart→login→catálogo→playback→eliminar con fixture, comprobar aislamiento/DB sin secretos/build/contrato y validar OpenSpec estricto; registrar evidencia sin credenciales y distinguir preparación local de publicación.

## Workflow follow-up

- Implementación local autorizada por la petición del usuario; sin nueva confirmación para apply.
- No ejecutar migraciones ni publicar en servicios anteriores o producción. Publicación futura exige autorización y destino verificado.
- Rotación de claves, múltiples perfiles y escala son ampliaciones posteriores; la clave original es necesaria para recuperar fuentes guardadas.

## Evidence

Resultados completos en `docs/source-persistence-verification.md`: migración local 002 y datos previos conservados; 26 casos/subcasos Go con -race más las dos integraciones PostgreSQL ejecutadas también con -race; builds, Compose y revisión independiente correctos. Navegador real autorizado: guardado, reinicio, logout/login y reproducción con credenciales recuperadas, sin reentrada de la URL. Borrado y concurrencia verificados con fixtures para conservar la fuente real solicitada. Sin publicación.
