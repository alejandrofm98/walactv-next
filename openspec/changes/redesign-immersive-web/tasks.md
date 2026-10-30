# Tasks

Cada tarea está acotada a un máximo aproximado de dos horas. El coordinador asigna propiedad de archivos antes de paralelizar.

## 1. Contexto y base
- [x] 1.1 Completar PRODUCT.md/DESIGN.md con la dirección recibida mediante Impeccable; cargar contexto y referencia product antes de editar UI.
- [x] 1.2 Registrar baseline del build y acceso, y revisar handlers/controles existentes que deben conservarse. La captura visual autenticada de biblioteca queda pendiente en 5.2.
- [x] 1.3 Definir tokens compartidos OKLCH, escala rem, espaciado, superficies, botones/campos, foco y estados semánticos; documentar decisiones y comprobar contraste.

## 2. Acceso y estructura
- [x] 2.1 Rediseñar acceso con formulario dominante, labels, errores y estados busy; conservar submit/autocompletado.
- [x] 2.2 Rediseñar shell escritorio/móvil y cuenta/fuente, preservando todos los destinos y haciendo explícita la acción de cerrar sesión.
- [x] 2.3 Rediseñar sin fuente, recuperación, conexión y confirmación de borrado con privacidad y estados actuales.

## 3. Descubrimiento y biblioteca
- [x] 3.1 Rediseñar directo: logos contenidos, títulos legibles, filtros, sincronización, carga/vacío/error y paginación.
- [x] 3.2 Rediseñar películas/series con carteles, fallback, búsqueda/categoría/idioma y seguir viendo, manteniendo sus handlers y estado.
- [x] 3.3 Rediseñar Mi lista y Ace Stream con estados/acciones existentes y explicaciones de compatibilidad claras.
- [x] 3.4 Rediseñar ficha, temporada/episodios y confirmaciones; comprobar scroll, foco, Tab, Escape y retorno de foco con flujo sintético.
- [x] 3.5 Diferenciar el catálogo de Nuvio/Stremio con un encabezado editorial propio y controles directos, sin alterar la navegación ni los datos.

- [x] 3.6 Reordenar todos los filtros compartidos a búsqueda → idioma → categoría; conservar valores/handlers y comprobar orden DOM, visual y Tab en Directo/Películas/Series.
- [x] 3.7 Retirar microcopy promocional/repetida de acceso, fuente, catálogo, Mi lista, Ace Stream, fichas y player; conservar ayuda, privacidad, compatibilidad, atribuciones y errores útiles.

## 4. Reproducción y adaptación
- [x] 4.1 Rediseñar reproductor, volver, título, carga, inicio manual, fallo y conversión; conservar vídeo/controles/fullscreen/lifecycle actuales.
- [x] 4.2 Ajustar las superficies y estados representativos a móvil/tablet y 320px, con áreas táctiles y sin dependencia de hover. El zoom 200% queda pendiente de la revisión integral 5.2.
- [x] 4.3 Añadir transiciones de estado breves y tratamiento de movimiento reducido; evitar animaciones de layout/carga de página y recursos pesados.

- [x] 4.4 Diferenciar la presentación `live` como IPTV y `movie|episode` como VOD inmersivo inspirado en Nuvio/Stremio; conservar vídeo/controles/lifecycle y no añadir EPG, zapping, pistas/fuentes o episodios sin soporte.
- [x] 4.5 Ajustar ambos modos a escritorio/móvil y fullscreen; mantener carga, inicio manual, reintento y conversión VOD; eliminar controles de salida duplicados y texto decorativo.

## 5. Comprobación y entrega local
- [x] 5.1 Compilar web y comparar budgets/tamaño antes/después; validar OpenSpec estricto. Confirmar que Compose, variables, DB/migraciones y API no cambiaron.
- [ ] 5.2 Revisar visualmente todas las superficies protegidas a 1440×900 y 390×844, estructura a 768px/320px y zoom 200%; revisar estados con datos saneados. Pendiente: no hay sesión autenticada/fixtures disponibles para recorrer la biblioteca sin usar datos privados.
- [x] 5.3 Comprobar foco inicial, Tab/Shift+Tab, Escape y retorno en diálogos, contraste de tokens y movimiento reducido; corregir hallazgos.
- [ ] 5.4 Comprobar búsqueda/filtros/página→ficha/reproducción→volver, navegación de temporadas, favoritos/Ace y fuente/sesión sin alterar contratos. Pendiente de recorrido autenticado; no se accedió al IPTV durante el rediseño.
- [x] 5.5 Reconstruir solo web local si hace falta para mostrar resultado y registrar evidencia/límites en docs/local-verification.md. Entregar implementación local distinguida de push/despliegue.
- [x] 5.6 Revisar el encabezado anti-referencia en escritorio y móvil con una vista sintética de datos ficticios; actualizar evidencia local. La biblioteca real sigue pendiente en 5.2 por estar protegida y no contar con una sesión/fixture autorizada.

- [ ] 5.7 Comprobar las preferencias posteriores con estados sintéticos saneados: orden de filtros, textos concisos, directo, película, episodio y conversión; registrar preservación de controles/progreso/salida y límites de la revisión. Recompilar web y repetir validación OpenSpec tras los cambios; confirmar que no se alteran Compose, variables, API ni migraciones.
