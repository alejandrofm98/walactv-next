# Tasks

La primera implementación y la revisión de URLs españolas están completadas localmente. Los bloques 1–4 conservan su estado histórico y los bloques 5–7 registran esta revisión. Cada bloque es acotado a un máximo estimado de dos horas; dividirlo si supera ese límite. Esta planificación no autoriza publicación. No añadir ni ejecutar pruebas automatizadas ni aceptación manual salvo petición del usuario; build/compilación son comprobaciones permitidas.

## 1. Resolución privada por identidad

- [x] 1.1 Añadir GET puntual de item reutilizando propietario, generación activa y DTO saneado; distinguir 401, 404 y error transitorio sin cambiar esquema.
- [x] 1.2 Añadir contrato de consulta puntual al cliente Angular.

## 2. Rutas y fichas

- [x] 2.1 Añadir construcción central de rutas canónicas, lectura reactiva de parámetros y resolución de fuente/item independiente de listados; cancelar respuestas obsoletas.
- [x] 2.2 Conectar canales, fichas de película/serie y acciones de episodio a enlaces reales; conservar modal/foco, temporada y acceso en pestaña nueva.
- [x] 2.3 Implementar estados de carga, catálogo pendiente, no disponible y error con reintento, sin cambiar fuente ni buscar por título; manejar temporada inválida y rechazar episodio ajeno a la serie.

## 3. Reproducción e historial

- [x] 3.1 Hacer que rutas de reproducción creen playback con validación actual y progreso existente, también sin memoria previa; evitar apertura desde fichas o llamadas duplicadas.
- [x] 3.2 Conservar cierre/progreso/lease/transcodificación tanto al salir como al cambiar parámetros; cancelar aperturas pendientes y cerrar resultados tardíos.
- [x] 3.3 Conectar Atrás/Adelante, cerrar ficha, Escape y salida a sus destinos; conservar filtros/temporada de sesión y restaurar foco, incluyendo acceso directo sin historial interno.
- [x] 3.4 Mantener `/` y compatibilidad limitada de `/player` con reemplazo a canónica o aviso seguro sin selección.

## 4. Sesión y compilación

- [x] 4.1 Recuperar destino después del login con validación estricta de ruta interna; limpiar contexto al cambiar fuente/usuario o terminar sesión y reautorizar siempre desde Go.
- [x] 4.2 Ejecutar build web y compilación Go. Registrar que no cambian Compose/variables/esquema y que no hay migraciones nuevas, push ni despliegue.

## 5. Identidad y formato españoles

- [x] 5.1 Centralizar las rutas exactas `/directo`, `/peliculas`, `/series`, `/ver` y `/episodios`; usar slug antes del ID decimal con análisis por último guion y descriptor tNeM de episodio; generar slugs desde título IPTV saneado con algoritmo y límites del diseño.
- [x] 5.2 Calcular origen compacto determinista con Web Crypto desde source.id; generar `o` obligatorio y resolver la fuente única desde sesión, rechazando origen ausente/distinto y estados de fuente inconsistentes sin cambiar API, esquema ni variables.
- [x] 5.3 Validar gramática de slug/sufijo ID decimal/descriptor de episodio/query, rechazar separadores codificados y normalizar slugs válidos desactualizados por reemplazo; no usar slug como búsqueda, identidad ni autorización.

## 6. Integración y compatibilidad

- [x] 6.1 Actualizar enlaces de catálogo/fichas/reproducción y salidas a las rutas españolas con origen; conservar generación reactiva y cierre de recursos ante cambio real de identidad, sin reiniciar playback al corregir slug/query.
- [x] 6.2 Cambiar query canónica a `temporada`, conservar selección y pertenencia de episodio, derivar su temporada real para regreso y normalizar valores inválidos mediante reemplazo.
- [x] 6.3 Actualizar returnUrl con reconocimiento exacto de rutas y queries españolas/legacy, preservando origen tras login y rechazando duplicados, destinos externos y equivalentes codificados.
- [x] 6.4 Añadir redirección de rutas técnicas anteriores tras autorizar sourceId exacto y resolver IDs/títulos, traducir `season`, preservar historial/contexto y evitar doble playback; mantener compatibilidad limitada de `/player`.
- [x] 6.5 Invalidar lecturas/aperturas/contexto al sustituir fuente o sesión; nunca reabrir un enlace antiguo contra el ID coincidente de la fuente nueva.

## 7. Compilación de la revisión

- [x] 7.1 Ejecutar build web y compilación Go, sin añadir/ejecutar pruebas ni aceptación manual. Registrar ausencia de cambios de Compose, variables y esquema, de migraciones nuevas y de push/despliegue.
