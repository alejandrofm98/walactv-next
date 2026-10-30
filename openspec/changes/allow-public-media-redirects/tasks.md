## 1. Transporte de medios

- [x] 1.1 Separar validación/dial público de medios del control con allowlist; fijar conexión a IP validada, preservar TLS y deshabilitar proxy ambiental.
- [x] 1.2 Aplicar política a raíz/assets y cada redirect: procedencia autorizada, GET/HEAD, HTTP:80/HTTPS:443, URL válida, DNS público, máximo tres saltos, secretos y errores saneados.

## 2. HLS y regresiones

- [x] 2.1 Permitir assets públicos multidominio derivados de manifiestos, resolver desde URL final y mantener reescritura opaca, límites, aislamiento y cancelación.
- [x] 2.2 Añadir pruebas deterministas de CDN no listado y HLS con variantes/KEY/MAP/segmentos redirigidos, GET/HEAD/Range/206/416, revocación y usuario ajeno.
- [x] 2.3 Probar bloqueos SSRF IPv4/IPv6, DNS mixto/vacío/rebinding, IP validada en dial, esquemas/puertos/métodos/userinfo inválidos, límite/loop de redirects y ausencia de cabeceras privadas; conservar tests de control restringido.

## 3. Integración local y documentación

- [x] 3.1 Documentar semántica de `IPTV_ALLOWED_HOSTS`, compatibilidad/rollback sin migraciones ni servicios/variables nuevos; no copiar secretos ni cambiar entornos externos.
- [x] 3.2 Ejecutar pruebas Go relevantes con race, OpenSpec estricto y Compose config sin secretos; reconstruir API local y comprobar salud.
- [x] 3.3 Coordinador: verificar reproducción real local secuencial sin depender de inscribir manualmente el CDN observado, cerrar playback y registrar evidencia saneada y limitaciones. Sin push/despliegue productivo.
