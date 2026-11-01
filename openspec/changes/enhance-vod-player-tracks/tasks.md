# Tasks

Tareas acotadas a sesiones de hasta dos horas; conservar trabajo de otros agentes. No publicar. Contratos y límites exactos en spec/design.

## 1. Inspección privada de archivos

- [ ] 1.1 Añadir inventario efímero/revisión por playback y contrato POST tracks autorizado, gated por VOD_TRANSCODING_ENABLED; probar propietario, file-only, deshabilitado, caché y revisión/IDs opacos sin metadata sensible.
- [ ] 1.2 Reutilizar transición de drenaje, gateway y slot global para ffprobe con límites de 10s/16MiB/32 requests/256KiB JSON/64 streams; probar con proveedor simulado que no hay entradas simultáneas, no se descarga archivo completo y timeout/cancelación liberan proceso/listener/slot.
- [ ] 1.3 Sanear inventario y clasificar audio decodificable/subtítulos textuales/bitmap/indisponible; probar MP4 con metadata final, MKV, cero audio, idiomas ausentes/duplicados, límites y etiquetas maliciosas. Mantener restauración de posición/pausa tras probe y documentar matriz/limitaciones de inspección.

## 2. Selección de audio en conversión

- [ ] 2.1 Extender POST/vista transcode con revisión e IDs opcionales validados antes de cancelar; resolver mapas solo servidor y actualizar idempotencia por selección/offset. Probar clientes antiguos sin campos, IDs ajenos/manipulados, tipo incorrecto y revisión obsoleta.
- [ ] 2.2 Mapear audio elegido y reiniciar generación desde posición lógica; conservar selección en seek/reanudación larga y estado playing/paused. Probar con dos tonos/idiomas simulados, ausencia de audio, mismo offset con distinta pista, saturación y restauración tras fallo.

## 3. Subtítulos textuales acotados

- [ ] 3.1 Verificar ffprobe, WebVTT y decoders textuales en imagen API; añadir segundo output WebVTT del mismo FFmpeg por pipe privado con flush y mapas explícitos. Probar fixture SRT/ASS/mov_text, salida incremental antes del fin del archivo y una sola entrada upstream. Si hay bloqueo de flush, entregar evidencia y usar la alternativa segmentada del mismo proceso descrita en design, sin extracción completa ni otro proceso lector.
- [ ] 3.2 Implementar parser servidor de cues completos, normalización a tiempo lógico y ventana VTT autorizada por generación, no-store y límites de 512 cues/2MiB/16KiB por cue; probar texto malicioso, cue inválido/denso, propietario ajeno, generación revocada y escena larga sin diálogo sin falso stall A/V.
- [ ] 3.3 Integrar cancelación, cierre de pipe/listeners y limpieza de caché/cues con seek, cambio de pista, logout, eliminación de fuente y shutdown; probar con race que no hay deadlock ni recurso/fetch obsoleto y que se mantienen presupuestos totales y kill de 3s.
- [ ] 3.4 Documentar formatos textuales soportados y pérdida de estilos ASS, bitmap sin OCR, compatibilidad de campos opcionales y rollback sin migración; verificar que Compose/variables/presupuestos no cambian y que feature deshabilitada conserva reproducción default.

## 4. Estado y controles VOD

- [ ] 4.1 Añadir adaptación de pistas hls.js 1.7.3 y nativas controlables con eventos de grupo/lista/selección y limpieza por intento; probar pistas tardías, APIs ausentes, idiomas ausentes/duplicados y rechazo sin confirmar cambios fallidos ni usar índices anteriores.
- [ ] 4.2 Integrar petición explícita de inventario de archivo con pausa/drenaje/restauración, error ocupado/parcial/deshabilitado y confirmación de conversión hasta 720p al seleccionar; probar que MP4/MKV sin API nativa permite descubrir y elegir pistas soportadas sin crear otro lease.
- [ ] 4.3 Conectar selección backend/audio/subtítulos y polling de ventana VTT cada 2s, parser cliente acotado/VTTCue con ajuste por offset, descarte de fetch/cues antiguos y Desactivados inmediato; probar sincronía tras offset no cero, seek, pausa larga, retry, menú y salida, sin innerHTML ni solicitudes de URLs de cues.
- [ ] 4.4 Aplicar composición VOD propia con vídeo dominante, título, salida única, play/pause, volumen/silencio, timeline y fullscreen; preservar timeline lógico de conversión, duración desconocida, errores/inicio manual y fallback de controles nativos. Verificar película/episodio y que Live conserva layout/función anteriores.
- [ ] 4.5 Añadir menús solo con selección real, etiquetas/estado confirmado e información concisa de límites; implementar ocultación 3s con excepciones foco/pausa/menú/carga/error. Verificar teclado/toque, primer toque operable, foco visible/Escape, cues legibles y sin barras duplicadas.
- [ ] 4.6 Ajustar 320/390/768/1440px, zoom 200%, safe areas, movimiento reducido, objetivos 44px y contraste; verificar fullscreen de contenedor y fallback nativo, sin scroll horizontal. Documentar capacidades observadas por navegador.

## 5. Integración determinista

- [ ] 5.1 Añadir fixtures de HLS multiaudio/WebVTT y tests del reescritor EXT-X-MEDIA relativo/absoluto/anidado, autorización y revocación existentes; ejecutar tests Go focalizados sin ampliar SSRF ni transporte por defecto.
- [ ] 5.2 Probar en navegador real cambios audibles/cues sincronizados en HLS y MP4/MKV convertido con pista elegida; incluir pista fallida, bitmap no soportado, probe agotado y API anterior 404. Registrar motor/navegador y distinguir verificaciones simuladas de reproducción real.
- [ ] 5.3 Ejecutar recorrido integrado de reproducción directa/conversión habilitada/deshabilitada, selección durante pausa, seek/reanudación/progreso, fullscreen, salida/filtros y logout; verificar Live antes/después y ausencia de datos privados. El proveedor real lo coordina el principal.
- [ ] 5.4 Ejecutar Go tests relevantes con race, build web, build/comprobación de capacidades de imagen API y `openspec validate enhance-vod-player-tracks --strict`; registrar resultados, riesgos de plataforma y revisión sin push/despliegue.
