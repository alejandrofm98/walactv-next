## Context

Actualmente `proxyMedia`, `CheckRedirect`, `dialPublic` y `opaqueAsset` comprueban la misma allowlist. El control usa además origen fijo. Las reproducciones generan la raíz desde una fuente previamente autorizada y solo entregan IDs de assets opacos al navegador. HLS ya resuelve referencias contra `resp.Request.URL`, limita manifiestos a 5 MiB, 4096 referencias por manifiesto y 8192 assets por playback.

## Goals / Non-Goals

Permitir la topología pública dinámica del proveedor sin convertir la API en un proxy abierto. Conservar aislamiento, contratos, Range, HEAD, cancelación y límites actuales. No ampliar políticas de control, catálogo, arte o TMDB ni admitir destinos/puertos privados.

## Decisions

1. **Autoridad por procedencia.** La raíz sigue construyéndose desde proveedor permitido y contenido autorizado. Solo `Location` de una petición de medios autorizada y referencias de un manifiesto obtenido para ese playback pueden introducir destinos adicionales. El navegador nunca envía una URL upstream ni adquiere autorización para un dominio entero. El asset de un playback no sirve en otro playback/usuario.
2. **Transporte de medios separado.** Extraer/reutilizar la comprobación de IP pública sin retirar allowlist del dial de control. El cliente/dial de medios permite host público no inscrito. Control y arte conservan su comportamiento. Desactivar proxies del entorno.
3. **Validación por salto y conexión.** Admitir únicamente HTTP:80 o HTTPS:443, incluidos puertos implícitos, host no vacío, GET/HEAD, sin userinfo ni fragmento/zone ID. Resolver DNS para cada destino, rechazar respuesta vacía o cualquier dirección privada/reservada, IPv4 mapeada o IPv6 según política pública existente. Conectar a una IP del conjunto validado, conservando Host y TLS ServerName/certificado; no permitir una resolución diferente sin nueva validación. Reutilizar una conexión establecida a IP pública es válido; cada conexión nueva debe validar de nuevo. Los redirects no eluden la validación inicial ni los límites. Mantener compatibilidad HTTP/HTTPS existente, incluida transición entre ambos; TLS se valida siempre en HTTPS.
4. **Límites y secretos.** Máximo tres redirecciones por petición; cuarto salto/loop devuelve error saneado y cierra bodies. Solo reenviar Range/If-Range permitidos; no Authorization, Cookie, Proxy-Authorization ni cabeceras de sesión/hop-by-hop. Conservar únicamente las credenciales/tokens que ya forman parte de URLs del proveedor o de `Location`/URI explícitos, en memoria, sin copiarlos artificialmente a otro destino. No exponer ni registrar URL upstream o errores con secretos.
5. **HLS completo.** Aceptar dominios públicos nuevos tanto en URI absolutas de variantes, segmentos, KEY, MAP y demás atributos soportados como en redirecciones de cada asset. Resolver relativos contra la URL final del manifiesto, reescribir a assets opacos y validar antes de conectar. No descargar segmentos al reescribir ni crear requests paralelas para cada URI. Un recurso peligroso produce error saneado sin abrir conexión prohibida; no ensanchar límites de manifiesto/assets existentes.

## Risks / Trade-offs

El proveedor puede dirigir una reproducción hacia cualquier destino público estándar: es una ampliación deliberada de confianza limitada a medios autenticados y vida del playback. La validación DNS/IP, ausencia de URL libre, puertos, métodos y límites reducen SSRF; no aseguran disponibilidad ni compatibilidad del archivo. No persistir automáticamente los CDN en configuración/BD.

## Migration Plan

Sin cambios de esquema ni migraciones. Compose conserva servicios, volúmenes y orden actual de migrador/bootstrap; ejecutar sus pasos idempotentes como de costumbre. Actualizar documentación y `.env.example` únicamente con nombres/semántica. API/web mantienen contrato. Rollback de imagen conserva datos y vuelve a exigir allowlist manual para CDN. Comprobación real solo local, secuencial, por coordinador y sin editar producción ni publicar.

## Validation

Fixtures con DNS/dial controlables permiten probar hosts públicos nuevos sin red externa ni habilitar red privada en runtime. Cubrir éxito de archivos/HLS dinámicos, raíz/control no permitido, cada bloqueo SSRF, conexión a IP validada, rechazo DNS mixto y rebinding, tres/cuatro redirects/loop, headers, GET/HEAD/Range/206/416, revocación y cross-user. Ejecutar pruebas Go relevantes con race, validación OpenSpec y Compose sin secretos; coordinador comprueba salud y reproducción real breve en local.
