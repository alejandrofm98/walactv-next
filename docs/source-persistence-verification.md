# Persistencia privada de IPTV · comprobación local

Fecha: 2026-10-28. Cambio OpenSpec: `persist-private-iptv-sources`. Aplicación local: `http://127.0.0.1:8081`. No se ha publicado ni modificado ningún servicio/base anterior o productivo.

## Almacenamiento implementado

PostgreSQL existente del proyecto. Tabla `sources`: una fuente por propietario, ID estable al recuperar y `secret_ciphertext` con URL original, base, usuario y contraseña cifrados en Go mediante AES-256-GCM. Nonce aleatorio y AAD versión/propietario/ID impiden reutilizar el ciphertext en otro registro. La clave base64 de 32 bytes vive en `IPTV_ENCRYPTION_KEY`, fuera de la base y Git, solo en el entorno API. No se devuelve al navegador.

`source_catalogs` guarda snapshots JSONB de categorías/canales/títulos/episodios consultados, sin URLs privadas ni direct_source, conservando IDs/tipo/extensión/contentKey. No descarga de golpe todo el proveedor. Caché RAM/PG 60 s; al caducar se actualiza automáticamente. Si falla el proveedor, no se presenta un snapshot caducado como actual.

Logout revoca streams/RAM pero conserva guardado. «Eliminar fuente guardada» borra fuente y catálogo; favoritos/progreso permanecen. Una cuenta recuperada se revalida antes de reproducir; los leases siguen en RAM y requieren una sola réplica.

## Migración y entorno

- Backup privado previo en `artifacts/backups/before-saved-sources.dump`; backup posterior con ciphertext en `artifacts/backups/with-saved-sources.dump`. Ambos ignorados por Git y con permisos 600. La clave no se incluye en ellos.
- Migración 002 aditiva aplicada mediante `walactv migrate`, más repetición y dos migradores concurrentes con código 0; versiones `[1,2]`.
- Antes/después de migrar: un usuario, un favorito y tres progresos; sin pérdida de datos previos.
- `setup-local.sh`: generación nueva base64 válida; segundo arranque conserva el archivo/clave exactos; clave ausente en archivo existente produce error y no genera sustituta.
- Arranque API sin clave y con clave malformada: rechazado sin modificar datos. Clave válida pero equivocada/ciphertext alterado/AAD ajeno: pruebas de denegación, sin borrado/sobrescritura.
- Compose `config --quiet`, shell syntax y builds Go/Angular correctos. `.env` modo 600 y excluido del build. Escaneo de archivos versionables sin coincidencias con las credenciales reales ni secretos generados del entorno.

## Pruebas automatizadas

- Suite Go con `-race`: 26 casos/subcasos correctos; dos pruebas PostgreSQL se omiten cuando no existe DSN explícito de prueba. `go vet` y build correctos.
- Ambas pruebas PostgreSQL optativas ejecutadas contra esta base local, mediante loopback en su namespace de red, con fixture de proveedor sin conexión externa. Incluyen sesión/CSRF, aislamiento A/B, ciphertext, restauración con nuevo API, logout conserva fila, catálogo saneado, clave incorrecta no sobrescribe y borrado en cascada.
- Incluyen regresiones con proveedor bloqueado durante logout y fetch de catálogo bloqueado durante DELETE: no se restaura RAM, fila ni catálogo revocados.
- Integración PostgreSQL con `-race` ejecutada también: ambos tests correctos, incluyendo los casos concurrentes de logout/DELETE; código de salida 0.
- Revisión independiente: hallazgos corregidos y cierre sin observaciones pendientes. La web se comprueba mediante build y navegador; no se añade un ejecutor unitario Angular.

## Navegador y fuente real autorizada

Se introdujo la fuente una vez en la versión nueva para guardarla. Después:

1. La interfaz mostró «Fuente guardada» y eliminó el campo privado del formulario.
2. Inspección de base: una fuente, ciphertext de 251 bytes y snapshots de catálogo sin URLs/campos de credenciales en claro; solo se imprimieron conteos, nunca el contenido secreto.
3. Reinicio del contenedor API y recarga web: fuente recuperada automáticamente, sin pedir URL.
4. Logout y nuevo login: fuente recuperada de nuevo, con biblioteca conservada.
5. Selección de TDT y reproducción del canal con las credenciales recuperadas: 1280×720, estado ready=4, sin pausa y avance a aproximadamente 55 s.
6. Reproducción detenida y aplicación preparada con la fuente guardada. El borrado se verificó con fixtures de propietarios temporales para conservar la configuración real solicitada por el usuario.

La captura sin credenciales se conservó fuera del repositorio.

## Recuperación y límites

Conservar backup de PostgreSQL y la clave original en custodia privada externa. Un backup solo de base no puede descifrar las fuentes sin esa clave. No hay rotación automática ni regeneración silenciosa. Rollback de imagen no elimina tablas: la versión anterior vuelve al comportamiento efímero; una compatible con la clave original recupera el guardado.

Persistir configuración no elimina el límite de conexiones del proveedor ni garantiza disponibilidad de sus servidores. Los ensayos prolongados, dispositivos reales, backup/restore productivo y publicación/Dokploy siguen como tareas futuras del proyecto inicial.
