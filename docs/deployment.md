# Contexto de despliegue

## Revalidación actual de Dokploy · 2026-11-01

Lectura del panel Dokploy v0.30.10, solo lectura: `WALACTV NEXT` mantiene el entorno `production` con `DB NEXT` y `WEB NEXT`. `WEB NEXT` apunta al repositorio `alejandrofm98/walactv-next`, rama `main`, Compose `./compose.dokploy.yml`; Auto Deploy está activado con trigger `On Push`. El historial del panel muestra como último despliegue correcto el commit `a83c78d` y el anterior `c243fc50`. Por tanto, el push autorizado a `origin/main` actualizará el entorno de producción. No se modificó la configuración de Dokploy ni se consultaron valores de secretos.

## DB NEXT · 2026-11-01

Decisión posterior del usuario: no desea configurar copias de seguridad para esta instalación. Las indicaciones históricas de aplazar la importación hasta configurar backups ya no reflejan su decisión; no hay backup/restauración comprobados.

Actualización posterior: `WEB NEXT` desplegó correctamente el commit `c243fc50f61c090dca225398b8be2e44e675753e`. Dokploy mostró web y API saludables, y los trabajos de migración y bootstrap con código 0; el dominio HTTPS respondió HTTP 200. No se verificó el inicio de sesión ni se importó una fuente IPTV. Este resultado sustituye al estado de fallo del primer intento descrito abajo.

El usuario creó `DB NEXT` en Dokploy y se inició PostgreSQL independiente para producción. Se observó el contenedor en ejecución con `postgres:18`. Por solicitud y confirmación del usuario, se guardó el puerto externo TCP `5439` en el recurso; las seis bases anteriores tenían configurados los puertos 5433–5438. El usuario configurará el firewall del VPS. Queda pendiente comprobar la publicación efectiva del puerto y la conectividad externa y de la aplicación; el valor guardado no demuestra que sea accesible. No se han configurado ni probado copias de seguridad para Next.

El primer despliegue manual de `WEB NEXT` usó el commit `315fc5d0551b59ebae09c16c889f4c48a7d7cfca` y terminó en error. El trabajo `migrate` terminó con código 0 y `bootstrap-admin` con código 1: el log indicó que las credenciales iniciales no cumplían las longitudes requeridas. API y web quedaron creadas sin iniciar. El dominio elegido quedó configurado el 2026-11-01 en Dokploy para `web:80`, ruta `/` y HTTPS con Let's Encrypt; el panel indica que hace falta redesplegar para aplicarlo. Falta corregir privadamente la contraseña bootstrap (12–72 bytes), desplegar de nuevo y comprobar salud, certificado y ruta. No se importó la fuente IPTV real.

## Actualización de retirada · 2026-11-01

Por decisión expresa del usuario se eliminaron en Dokploy los servicios Compose `WEB` e `IPTV` de producción e `IPTV DEV`, marcando en los tres la opción de eliminar los volúmenes asociados. Dokploy confirmó la eliminación de cada servicio. Los volúmenes externos `iptv-data` y `walactv-dev-iptv-data` siguen presentes: Docker rechazó su eliminación porque hay contenedores que los usan. Los listados del panel conservan `SCRAPPER` y `db` en producción, y `SCRAPPER DEV` y `DB DEV` en desarrollo. No se eliminaron las bases de datos ni los scrapers.

La web anterior ya no está disponible como servicio de Dokploy. La comprobación previa a publicar Next observó que aún no había despliegues; esa instantánea quedó supersedida por la revalidación actual de arriba, que confirma despliegues correctos de `WEB NEXT`. La API antigua ya no está disponible para Android. Las observaciones siguientes del 30 de octubre son históricas y no describen el estado actual de esos tres servicios.

## Snapshot histórico de despliegue · 2026-10-30 (supersedido por la revalidación del 2026-11-01)

- El repositorio público [walactv-next](https://github.com/alejandrofm98/walactv-next) se inicializó en `main` con el commit `27cd12f5dc73759070cc710e77c360488bd713e8`. Antes del push se comprobó que el remoto estaba vacío, no había refspec de push y ningún servicio Dokploy de Next estaba asociado.
- Se creó el proyecto Dokploy `WALACTV NEXT`, entorno `production`, y el servicio Compose `WEB NEXT`. Está vinculado al repositorio público, rama `main` y `compose.dokploy.yml`; Auto Deploy está apagado mientras se completa la configuración. Todavía no se ha ejecutado ningún despliegue de Next.
- Dokploy v0.29.5 se revisó con sesión autenticada el 2026-10-30, en modo de solo lectura. El proyecto WALACTV contiene un entorno de producción y uno de desarrollo, con siete recursos en total. La configuración privada por servicio, IDs, hostnames y revisiones recuperables se conserva en el inventario privado local de Dokploy; este documento público no replica esos detalles.
- La web anterior está en producción, tiene HTTPS y sigue ejecutándose. Su Auto Deploy está habilitado con activador `On Push`; la revisión de despliegues muestra una última versión exitosa recuperable. No se cambió su configuración ni su dominio.
- El usuario eligió como dominio final el mismo hostname que ahora sirve la web anterior. La referencia exacta permanece en el inventario privado. Se transferirá desde la web antigua a Next solo cuando el nuevo servicio esté desplegado y sus comprobaciones internas hayan pasado.
- Las API IPTV y los servicios scraper de producción y desarrollo siguen activos. Los recursos PostgreSQL antiguos también existen; Dokploy no muestra proveedor de copias de seguridad configurado para las bases observadas.
- El usuario decidió crear la PostgreSQL propia con el recurso Database → PostgreSQL de Dokploy, separado de Compose. Todavía no se ha creado: el usuario introducirá la contraseña y los valores de secretos en el panel; este agente no los generará ni cargará. No hay destino S3/R2 configurado; por decisión del usuario, la preparación de backups queda pendiente hasta inmediatamente antes de importar la lista IPTV real. No se importará esa lista antes de configurar y comprobar el backup.
- El dominio sigue en la web anterior; no se ha transferido ni se ha desplegado Next. Ningún servicio o dato legacy se modificó o eliminó.

## Destinos pendientes · snapshot histórico del 2026-10-30 (supersedido)

La tabla siguiente conserva la fotografía previa a configurar `DB NEXT` y desplegar `WEB NEXT`; sus estados de Auto Deploy, PostgreSQL y primer despliegue ya no describen el entorno actual. Para el destino y Auto Deploy actuales, usar la revalidación Dokploy fechada 2026-11-01 al inicio de este documento (`WEB NEXT` en `main`, `compose.dokploy.yml`, producción y `On Push`). No se modificó la configuración del panel durante esta implementación.

| Elemento | Estado | Evidencia necesaria |
|---|---|---|
| Repositorio | Público, `main` publicado en el commit documentado | Antes de nuevos pushes, comprobar refspec y todos los destinos enlazados |
| Proyecto/servicio nuevos | `WALACTV NEXT` / `WEB NEXT`, entorno `production`, configurados en Dokploy | Completar PostgreSQL y secretos; no se ha ejecutado despliegue |
| Rama y Compose nuevos | `main`, `./compose.dokploy.yml` | Verificar que el checkout del servidor use este entrypoint al desplegar |
| Auto Deploy nuevo | Apagado; trigger `On Push` guardado | Habilitar después de confirmar env vars, base y comportamiento del primer despliegue |
| Web anterior | Revalidada en el panel el 2026-10-30 | Consultar el inventario privado para su dominio, ruta, repositorio, rama y última revisión exitosa antes del corte |
| PostgreSQL nueva | El usuario eligió Database → PostgreSQL de Dokploy; aún no se ha creado. El usuario introducirá la contraseña | Crear recurso independiente de Compose; completar red y secretos. Configurar y comprobar backups justo antes de importar la lista IPTV real |
| Dominio final | Elegido por el usuario: hostname actual de la web anterior | Verificar hostname/certificado en Dokploy al configurar Next; reasignarlo solo en el corte y mantener la web anterior disponible para recuperación |

El Compose productivo es `compose.dokploy.yml`, que extiende las definiciones de `compose.yml`. Usa la red `dokploy-network` (o el nombre confirmado mediante `DOKPLOY_NETWORK_NAME`) para permitir acceso interno a PostgreSQL y al proxy. `compose.local.yml` es solo para desarrollo y nunca se combina en Dokploy. Consulta el [runbook de publicación y recuperación](dokploy-release.md).

## Inventario legacy · snapshot del 2026-10-30 (histórico; parcialmente supersedido)

Esta fotografía se tomó en la interfaz autenticada de Dokploy el 2026-10-30. La retirada de servicios descrita en la sección del 2026-11-01 al inicio de este documento la supersede donde difieran estos estados. La tabla solo indica actividad y configuración observadas entonces, no métricas de tráfico ni prueba completa de consumidores; revalidar en el panel antes de actuar.

| Recurso | Evidencia actual | Evaluación |
|---|---|---|
| Web anterior de producción | Sigue ejecutándose; HTTPS y Auto Deploy `On Push` activos; última revisión exitosa disponible | Candidata a retirar únicamente tras desplegar Next, transferir la ruta, completar aceptación y ventana de recuperación |
| API IPTV de producción | API, Nginx y extractor ejecutándose; API y extractor saludables | Conservar: Android documenta el contrato de autenticación, catálogo, reproducción, favoritos y progreso |
| Scraper de producción | Nueve contenedores de trabajos/sincronización ejecutándose; se vio un despliegue fallido reciente, pero los contenedores actuales siguen activos | Conservar: alimenta datos usados por la API anterior; revisar fallos sin interrumpir trabajos |
| API IPTV y scraper de desarrollo | Ambos servicios y sus contenedores están ejecutándose | No declarar sin uso por falta de métricas; confirmar consumidores, tareas, volúmenes y responsables antes de retirar |
| PostgreSQL de producción y desarrollo | Ambos recursos existen; no se encontró proveedor de copias configurado | Conservar hasta verificar conexiones, datos, retención y restauración; no reutilizarlos para Next |
| Volúmenes y redes compartidos legacy | Las definiciones Compose locales muestran recursos compartidos por API/scraper | Conservar hasta comprobar montajes y consumidores reales en el host |

Las dos instancias de scraper y las dos API tienen Auto Deploy `On Push`. Una rama legacy puede afectar más de un servicio/entorno; no publicar ahí para preparar WalacTV Next. La actividad actual no autoriza cambios de configuración ni borrado. No se ha eliminado ningún elemento.

Estado actual según lectura del panel Dokploy del 2026-11-01: `WALACTV NEXT` / `WEB NEXT` están en producción, `main` y `compose.dokploy.yml`, con Auto Deploy `On Push`; la última revisión exitosa visible era `a83c78d`. `DB NEXT` es PostgreSQL externa a Compose. La configuración y salud del despliegue no autorizan publicar cambios ni modificar servicios. API y base no publican puertos; la instalación local continúa usando `compose.yml` más `compose.local.yml` en red aislada.

El despliegue mantiene `API_REPLICAS=1`; `VOD_TRANSCODING_ENABLED` queda en `false` hasta comprobar capacidad productiva. `DATABASE_URL`, `APP_ORIGIN`, `SESSION_SECRET`, `PROVIDER_IDENTITY_KEY`, `IPTV_ENCRYPTION_KEY`, `BOOTSTRAP_ADMIN_USERNAME`, `BOOTSTRAP_ADMIN_PASSWORD`, `IPTV_ALLOWED_HOSTS`, `TMDB_API_TOKEN` y `DOKPLOY_NETWORK_NAME` se configuran de manera privada en Dokploy. Registrar solo nombres/referencias. El nuevo sistema tiene usuarios y catálogo propios: no migra automáticamente cuentas, sesiones, fuentes, favoritos o progreso legacy.

## PostgreSQL y migraciones

- Desarrollo usa una base nueva exclusivamente local. No se importa ni modifica información de aplicaciones anteriores.
- Producción de Next usa `DB NEXT`, recurso PostgreSQL de Dokploy separado de Compose y de las bases legacy. El puerto TCP externo `5439` está guardado en el panel; publicación efectiva y conectividad desde la aplicación no se han comprobado aquí. El usuario introduce los valores secretos de forma privada. Por decisión del 2026-11-01, no se configurarán copias de seguridad para esta instalación; no hay backup/restauración comprobados.
- Las bases legacy de producción y desarrollo se observaron en el panel, pero no tenían proveedor de copias configurado. No se inspeccionaron datos ni valores de conexión; su relación con los servicios debe verificarse antes de cualquier retirada.
- `DATABASE_URL` es el nombre del secreto de conexión. Sus valores nunca se documentan ni imprimen con Compose resuelto.
- `compose.yml` usa la conexión externa definida por `DATABASE_URL`. En el entorno local, `compose.local.yml` la reemplaza por PostgreSQL 17 aislada, dentro del volumen `walactv-local-postgres-data`, sin publicar el puerto de base de datos al host.
- Compose ejecuta `walactv migrate` y después `walactv bootstrap-admin` como trabajos puntuales antes de iniciar la API. El bootstrap usa `BOOTSTRAP_ADMIN_USERNAME` y `BOOTSTRAP_ADMIN_PASSWORD`, no imprime sus valores y debe ser idempotente para permitir reinicios del stack.
- Una actualización/rollback de imagen no revierte el esquema. Se preferirán cambios aditivos; la restauración de backups requiere un procedimiento comprobado antes de producción.

## Configuración

Variables: `DATABASE_URL`, `APP_ORIGIN`, `SESSION_SECRET`, `PROVIDER_IDENTITY_KEY`, `IPTV_ENCRYPTION_KEY`, `IPTV_ALLOWED_HOSTS`, `TMDB_API_TOKEN` (opcional), `API_REPLICAS`, `BOOTSTRAP_ADMIN_USERNAME` y `BOOTSTRAP_ADMIN_PASSWORD`. El usuario configurará en privado los valores productivos en Dokploy; guardar y compartir solo nombres de variables y referencias, nunca valores. `DATABASE_URL` apuntará al recurso Database → PostgreSQL nuevo. `TMDB_API_TOKEN` se configura solo como secreto runtime del servicio API; `.env.example` solo incluye el nombre vacío, nunca un valor real. No llega a builds, bundles, respuestas ni logs. Si no está configurado, la metadata indica `disabled` y catálogo/playback funcionan igual. `API_REPLICAS` debe permanecer en `1` mientras los leases, streams e importaciones activas de IPTV estén en memoria. El override local añade `LOCAL_POSTGRES_DB`, `LOCAL_POSTGRES_USER` y `LOCAL_POSTGRES_PASSWORD`. `./scripts/setup-local.sh` crea `.env` ignorado con permisos 600, genera secretos locales y no muestra sus valores. Conserva un `.env` existente y no genera token TMDB. `IPTV_ALLOWED_HOSTS` es opcional: vacío permite proveedores Xtream en cualquier dominio, sujeto a validación DNS/IP pública; alta y control conservan HTTP:80 y HTTPS:443. Una lista separada por comas conserva la restricción por host. No se incluye URL, usuario ni contraseña de IPTV. Las credenciales IPTV entran por POST y se cifran en Go antes de guardarse en PostgreSQL. `IPTV_ENCRYPTION_KEY` es base64 de exactamente 32 bytes, estable y externa a la base/Git; se pasa solo al proceso API, nunca a build args ni al navegador.

## Arranque local

Requisitos: Docker Compose v2 y OpenSSL. Desde la raíz del repositorio:

```sh
./scripts/setup-local.sh
docker compose -f compose.yml -f compose.local.yml up --build -d
docker compose -f compose.yml -f compose.local.yml ps
```

La web escucha solo en `127.0.0.1:8081`; Nginx entrega `/api/` a Go por la red interna y desactiva buffering en el proxy de medios. Los tiempos de lectura y envío en medios tienen un límite de inactividad de 60 segundos, no un límite total de duración. PostgreSQL y API no publican puertos al host. Consulta la contraseña bootstrap directamente desde el `.env` local; no la copies a documentación ni informes.

Para parar la aplicación y conservar datos: `docker compose -f compose.yml -f compose.local.yml down`. Para eliminar intencionalmente la base **local** y su volumen: añade `-v`. Esto no modifica ninguna base externa.

El Compose base omite PostgreSQL local y requiere `DATABASE_URL` del recurso PostgreSQL externo. El archivo `compose.dokploy.yml` es el punto de entrada productivo. No usar `compose.local.yml` en Dokploy. Estado de producción, Auto Deploy y última revisión: ver la revalidación del 2026-11-01 al inicio; los párrafos fechados en 2026-10-30 que decían que Auto Deploy estaba apagado y no había despliegue están supersedidos. No se documentan valores de secretos ni se hacen comprobaciones de conectividad a producción desde esta tarea.

## Comprobaciones

Tras implementar se verificarán los comandos Go, web, validación Compose sin salida de secretos y salud `/healthz`. Las pruebas de streams usan fuentes simuladas; las pruebas reales se harán de una en una, respetando el límite del proveedor. Registrar implementación, publicación y despliegue verificado por separado.

## Verificación local realizada · 2026-10-28

`docker compose -f compose.yml -f compose.local.yml up --build -d` crea el PostgreSQL independiente, ejecuta `walactv migrate`, `walactv bootstrap-admin` y después `walactv serve`. Los jobs terminaron con código 0; PostgreSQL/API/web saludables. Migradores concurrentes y bootstrap repetido comprobados. `nginx -t`, Compose `config --quiet` y `/api/v1/health` correctos. Ver `docs/local-verification.md` para evidencia y límites.

La información de Dokploy del párrafo histórico original de esta verificación local quedó supersedida por la revalidación del 2026-11-01 al inicio del documento. La comprobación local descrita arriba conserva su alcance y fecha; no representa el estado de despliegue actual.

## Actualización de persistencia privada · 2026-10-28

Cambio OpenSpec `persist-private-iptv-sources`: migración aditiva `002`, fuente cifrada por usuario y snapshots JSONB del catálogo saneado. El migrador recorre todas las versiones SQL pendientes con advisory lock y transacción por versión, antes de iniciar API. No se modifica la migración 001 ni se eliminan datos anteriores.

Antes de migrar la instalación local se creó un backup privado en `artifacts/backups/before-saved-sources.dump` (ignorado por Git); la clave nueva se generó en `.env` privado antes de existir fuentes persistidas. No se imprime ni se incluye en el backup. Antes de producción deberán verificarse backup/restore y custodia externa de la clave.

Una restauración necesita PostgreSQL **y la clave original**. La clave no se rota/regenera automáticamente. Si es incorrecta, se deniega el descifrado sin borrar el registro; si falta o su formato no es válido, API no arranca. `setup-local.sh` conserva `.env` y la clave existente; si falta en un entorno existente, devuelve error para evitar sustituciones accidentales.

Logout revoca RAM/medios pero conserva fuente/catálogo guardados. Eliminar una fuente borra su fila y catálogo de forma autorizada, conservando la biblioteca. Fuentes guardadas no restauran streams ni leases anteriores. Los límites de autenticación y la restricción a una réplica siguen vigentes.

Rollback: las tablas nuevas son aditivas y se conservan al volver de imagen; la versión anterior ignora la persistencia y vuelve al funcionamiento efímero. Volver a la imagen compatible y conservar la clave original recupera las fuentes. No ejecutar DROP/down de estas tablas ni interpretar el rollback de imagen como restauración de la base.

## Catálogo persistente · 2026-10-28

El cambio `improve-iptv-discovery` añade la migración aditiva 003 y mantiene el migrador previo al arranque. Antes de migrar la base local se creó `artifacts/backups/before-discovery.dump`, con permisos privados e ignorado por Git. Conservar también la clave `IPTV_ENCRYPTION_KEY` original: la copia de PostgreSQL por sí sola no permite recuperar los datos cifrados. Una imagen anterior ignora las tablas añadidas; volver de imagen no revierte la migración ni es una restauración de datos.

La importación guarda una generación completa antes de publicarla; una actualización fallida conserva la anterior. Se procesan respuestas de hasta 256 MiB, 500.000 elementos por tipo y 120 segundos por petición, con un límite de 10 minutos para el trabajo. El coordinador de importaciones y la caché de imágenes viven en memoria: **mantener `API_REPLICAS=1`**. El soporte para varias réplicas requiere coordinación y protección frente a escritores obsoletos compartidas. No se añaden variables obligatorias ni servicios a Compose.

Verificación local: migración aplicada, servicios sanos y fuente restaurada después de reiniciar. Una importación real guardó 45.669 canales, 210.381 películas y 45.620 series; se comprobaron búsqueda global, imágenes, ficha de película, temporadas y reproducción individual de un MP4 y un episodio MKV. La disponibilidad, duración y compatibilidad de las emisiones dependen del proveedor y del navegador. Quedan pendientes pruebas automáticas de integración de fallos de importación, reinicio, concurrencia y aislamiento del proxy de imágenes.

## Metadata TMDB bajo demanda · 2026-10-29

Migración aditiva 004 añade campos opcionales saneados al catálogo importado, caché canónica `es-ES`, caché de temporada, decisiones privadas por huella y enfriamientos persistentes globales (429/autenticación) y por clave hash (red/5xx/payload inválido, sin almacenar títulos). El API acepta como máximo 2 peticiones TMDB simultáneas, 4/s, 10 aperturas frías por minuto y una cola acotada; cada request externa tiene timeout de 5 s y máximo 2 MiB. La caché positiva dura 7 días y puede servirse stale hasta 30 días; los no-match/ambiguos duran 24 h. Ante ausencia/invalidación del token, saturación o fallo externo se conserva el contenido y playback IPTV. Las consultas se hacen solo al abrir ficha/temporada; nunca durante importación, listados ni reproducción.

Antes de aplicar 004 a la PostgreSQL local aislada, se creó `artifacts/backups/before-metadata.dump` con modo 0600, ignorado por Git. Conservar la clave `IPTV_ENCRYPTION_KEY` existente junto con el backup. Producción usa la PostgreSQL externa de Dokploy; requiere backup/restore verificados y autorización antes de migrar. Una imagen anterior ignora las tablas/columnas nuevas. Rollback de imagen no revierte esquema y no debe ejecutar `DROP`/down; la imagen nueva posterior reutiliza datos/caché compatibles. No hay push ni despliegue autorizados/verificados.

## Destinos dinámicos de medios

`IPTV_ALLOWED_HOSTS`, cuando contiene hosts, restringe las fuentes y el plano de control. Vacío o ausente permite cualquier dominio proveedor, sujeto a controles de DNS, IP pública y puertos. Los servidores públicos de medios derivados de una reproducción autorizada, incluidas redirecciones y referencias HLS, no necesitan añadirse individualmente. La API valida las direcciones resueltas y el puerto en cada nueva conexión, y limita las redirecciones. Esto amplía la confianza en los destinos públicos indicados por el proveedor. No requiere migraciones, servicios ni variables nuevos. Una imagen anterior vuelve a exigir la lista para alta y reproducción; antes de un rollback hay que configurar allí los dominios de las fuentes existentes.

## Diagnóstico y compatibilidad de medios VOD

Las reproducciones autorizadas admiten hasta ocho redirecciones y puertos HTTP 80, 8000, 8080, 8081, 8880 y 8888 o HTTPS 443 y 8443. El control del proveedor conserva sus propias restricciones. La API expone al propietario un diagnóstico saneado del último fallo de reproducción, sin URLs ni credenciales y sin volver a solicitar el vídeo. La compatibilidad de contenedor y códecs depende del navegador. No hay migraciones, servicios ni variables nuevos. Al volver a una imagen anterior se recuperan las restricciones anteriores y el diagnóstico genérico, sin cambios de base de datos.

## Conversión de vídeo local para navegadores

`VOD_TRANSCODING_ENABLED` habilita el fallback opcional para películas y episodios que el navegador no consigue decodificar. El valor por defecto es `false`; FFmpeg se instala en la imagen de la API y ejecuta únicamente dentro de ese contenedor. La API transmite la entrada desde la fuente IPTV validada por un gateway loopback temporal, sin poner la URL privada en argumentos, logs o variables del proceso FFmpeg. La salida es HLS H.264/AAC hasta 720p, temporal y no persistente. Canales en directo y listas HLS no se convierten.

Compose limita el servicio API completo a 2 CPU, 1536 MiB y 128 procesos; la salida efímera usa un tmpfs de 256 MiB. Solo se permite un trabajo de conversión global, sin cola y conservando el lease IPTV existente. Vídeo 4K HEVC puede no convertirse en tiempo real dentro de ese presupuesto y se detiene tras los límites de preparación/inactividad. La búsqueda reinicia la conversión en el offset solicitado; la barra completa solo aparece con duración fiable. La salida HLS rueda aproximadamente 48 segundos; si se pausa más de 45 segundos, reanuda regenerando desde la posición lógica guardada. Al salir, cerrar sesión, eliminar la fuente o detener la API se cancelan procesos y se borra el trabajo.

En instalaciones locales, establecer `VOD_TRANSCODING_ENABLED=true` en `.env` y reconstruir Compose para habilitarlo. Producción requeriría evaluar CPU/RAM y habilitar la imagen/variable solo después de medir allí. No requiere migraciones de base de datos ni puertos/servicios/volúmenes persistentes nuevos. Un rollback de imagen cancela y elimina conversiones temporales; PostgreSQL no cambia.

La misma opción habilita `Buscar pistas del archivo`: ffprobe usa el slot VOD y gateway loopback, con límites de 10 s, 16 MiB, 32 solicitudes y 256 KiB de salida JSON; el agotamiento se informa como inventario parcial. El inventario es efímero por playback. La imagen verifica ffprobe, muxer WebVTT y decoders textuales SubRip, ASS y mov_text. Las pistas ASS pierden estilos y PGS/VobSub se muestran como no seleccionables, sin OCR. La salida WebVTT comparte el proceso FFmpeg, su entrada, slot y cancelación, y se limita a 512 cues, 2 MiB totales y 16 KiB por cue. No hay cambios de Compose, variables, PostgreSQL externa de Dokploy ni migraciones. La entrega incremental y sincronía por contenedor aún requieren verificación con fixtures antes de habilitar el uso operativo.

## Descubrimiento Cinemeta · 2026-11-01

`add-cinemeta-iptv-discovery` añade un modo opcional de Películas/Series con cruce privado por IMDb exacto. No añade variables, secretos, servicios/volúmenes Compose ni migraciones; el migrador conserva las versiones actuales y PostgreSQL productiva sigue externa a Compose. La aplicación continúa con `API_REPLICAS=1`, que conserva cursores y caché efímeros en una sola instancia. TMDB `es-ES` continúa bajo demanda en fichas; Cinemeta no proporciona streams ni datos de reproducción.

El backend impone timeout de 5 s, máximo 2 MiB por respuesta, 2 solicitudes externas simultáneas, separación global de 4 solicitudes/s, tres páginas Cinemeta por acción, caché pública en RAM de 15 minutos con máximo 128 lotes/16 MiB y enfriamiento ante límites/fallos. Las solicitudes de descubrimiento se limitan a 10/minuto por usuario. Cursores privados duran 10 minutos, se ligan a usuario/sesión/fuente/generación/filtros y desaparecen al reiniciar la API. Resultados privados usan `Cache-Control: private, no-store`; el contenido IPTV sigue procediendo de la generación activa.

Una reversión de imagen solo descarta caché y cursores efímeros; no cambia ni revierte PostgreSQL. No hace falta ejecutar migraciones ni preparar restauración de base de datos para este cambio. Construcción y comprobaciones locales no publican código ni despliegan. `origin/main` tiene Auto Deploy de producción `On Push` según la revalidación del 2026-11-01; cualquier push requiere autorización y coordinación separada.

La medición de intersección prevista con fixtures locales (210.381 películas / 45.620 series) queda pendiente: en este checkout no hay fixture de ese tamaño ni se consultó una fuente real o PostgreSQL de producción. La consulta implementada es una única búsqueda parametrizada `source_id + active_generation + kind + imdb_id ANY(batch)` con categoría/idioma opcionales; no hace N+1 ni carga todo el catálogo en memoria. Si una medición local aislada supera 2 s, revisar primero el plan de esta consulta y un índice aditivo `(source_id,generation,kind,imdb_id)`; no ampliar el esquema sin evidencia y diseño de migración/rollback revisados.

Los contextos limitan memoria privada a 1 MiB y 2.000 versiones, con hasta ocho por usuario y 128 globales en LRU. Al alcanzar un límite se pide recomenzar; no se presenta como fin del catálogo. Los buffers/caché y enfriamientos son efímeros y no se serializan en PostgreSQL.

Revisión local del cambio · 2026-11-01: `go build ./...` y `npm run build` completaron correctamente; Compose local y productivo pasaron `config --quiet`, y `git diff --check` no informó problemas. No se ejecutaron pruebas automatizadas, fixtures, benchmarks, servicios ni health checks; tampoco se consultó Cinemeta con tráfico real o PostgreSQL de producción. No hay archivos de migración/Compose o nombres de variables cambiados por este cambio. La comprobación funcional y la medición de rendimiento con fixture aislado quedan pendientes de solicitud expresa.
