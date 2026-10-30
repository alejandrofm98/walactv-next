# Tasks

## 1. Contrato y persistencia

- [x] 1.1 Adoptar `docs/catalog-contract.md`; confirmar modelos/estados/rutas entre API y UI y documentar investigación Cinemeta con fuentes oficiales.
- [x] 1.2 Crear backup privado local previo y migración aditiva 003: generaciones, estado, categorías/items, índices y arte cifrado, sin tocar 001/002.
- [x] 1.3 Implementar lectura SQL propia de generación activa, búsqueda literal global, filtros/idiomas y paginación/contadores coherentes.

## 2. Sincronización

- [x] 2.1 Implementar POST refresh/GET estado y worker único por fuente/cuenta, llamadas secuenciales de categorías/items por tipo con límites y errores seguros.
- [x] 2.2 Publicar generación completa atómicamente; preservar anterior al fallar, limpiar staging/antiguas, invalidar trabajos al borrar/reemplazar y recuperar interrupciones al reiniciar.
- [x] 2.3 Conectar alta/restauración UI sin generación con primera sync, conservar reproducción/leases independientes y reconstruir recursos de playback con IDs/extensiones del catálogo validado.

## 3. Arte y navegación

- [x] 3.1 Extraer URL de arte a ciphertext y servir endpoint autenticado con cliente SSRF público, MIME/firma/tamaño/timeout/redirects/caché acotados.
- [x] 3.2 Añadir modelos y estado Angular: búsqueda con debounce/cancelación, «Todas», categorías/idiomas, paginación y fecha/refresh/reintento sin respuestas viejas.
- [x] 3.3 Mostrar logos/carátulas lazy con fallback y navegación Directos/Películas/Series; ficha honesta, temporadas/episodios y reproducción existente.

## 4. Verificación y operación

- [ ] 4.1 Comprobar fixtures de catálogo grande, filtros cruzados, idioma desconocido, acceso A/B, sync concurrente/fallida/borrada/reiniciada y ausencia de secretos plaintext.
- [ ] 4.2 Comprobar proxy de imagen con redirects/IP privada, MIME inválido, exceso tamaño y autorización antes de caché; comprobar reproducción sin lease adicional durante sync.
- [x] 4.3 Ejecutar checks Go/web y migración local una sola vez; verificar Compose/health y navegador con fuente guardada, imagen, búsqueda, filtros, película y episodio.
- [x] 4.4 Actualizar documentación de operación/verificación, registrar límites reales, validar OpenSpec estricto y revisión independiente; distinguir local de publicación pendiente.
