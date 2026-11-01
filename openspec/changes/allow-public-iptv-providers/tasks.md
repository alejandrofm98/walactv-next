## 1. Política de proveedores

- [x] 1.1 Implementar normalización de `IPTV_ALLOWED_HOSTS`: sin hosts efectivos permite proveedores públicos compatibles; lista no vacía conserva restricción exacta, sin wildcard/sufijos.
- [x] 1.2 Aplicar la política de forma coherente a alta, fuentes guardadas, control/dial y raíz de reproducción, conservando formato Xtream, puertos y origen fijo.

## 2. Seguridad y regresiones

- [x] 2.1 Añadir pruebas deterministas de configuración nil/vacía/espacios, lista restrictiva, alta de proveedor no inscrito y raíz de playback sin lista; conservar aislamiento, límites y errores sin secretos.
- [x] 2.2 Verificar dial público con IPv4/IPv6 privadas/reservadas/metadata y variantes privadas mapeadas, DNS mixto/vacío, IP validada fijada frente a rebinding, esquema/puertos inválidos y proxy ambiental desactivado.
- [x] 2.3 Cubrir redirecciones de control al mismo origen, rechazo de cambio de esquema/host/puerto, máximo tres saltos y ausencia de cabeceras de sesión privadas; conservar pruebas de CDN/HLS por procedencia.
- [x] 2.4 Probar restauración de fuente cifrada al retirar la lista y rechazo sin borrado cuando se introduce una restricción que la excluye, usando el harness de PostgreSQL local si procede.

## 3. Documentación y comprobación

- [x] 3.1 Actualizar `.env.example` y documentación de configuración/despliegue para explicar modo público por defecto, restricción opcional, alcance Xtream/puertos, compatibilidad y rollback sin migraciones ni nuevos servicios.
- [x] 3.2 Ejecutar pruebas Go afectadas con race, OpenSpec estricto y validación Compose sin salida de secretos; registrar resultados y límites.
- [x] 3.3 Revisión independiente de SSRF, redirecciones, restauración y despliegue; corregir hallazgos antes de entregar. Cualquier prueba real, push o despliegue queda a cargo del coordinador con autorización vigente.
