# Contexto de despliegue

## Estado comprobado · 2026-10-30

- El repositorio público [walactv-next](https://github.com/alejandrofm98/walactv-next) existe y `origin` apunta a él. Antes de la primera publicación se comprobó que no tenía referencias remotas ni refspec de push, y ningún servicio Dokploy de Next estaba asociado. `main` se usa para inicializar el repositorio público; la rama del servicio Dokploy se seleccionará explícitamente después.
- Dokploy v0.29.5 se revisó con sesión autenticada el 2026-10-30, en modo de solo lectura. El proyecto WALACTV contiene un entorno de producción y uno de desarrollo, con siete recursos en total. La configuración privada por servicio, IDs, hostnames y revisiones recuperables se conserva en el inventario privado local de Dokploy; este documento público no replica esos detalles.
- La web anterior está en producción, tiene HTTPS y sigue ejecutándose. Su Auto Deploy está habilitado con activador `On Push`; la revisión de despliegues muestra una última versión exitosa recuperable. No se cambió su configuración ni su dominio.
- El usuario eligió como dominio final el mismo hostname que ahora sirve la web anterior. La referencia exacta permanece en el inventario privado. Se transferirá desde la web antigua a Next solo cuando el nuevo servicio esté desplegado y sus comprobaciones internas hayan pasado.
- Las API IPTV y los servicios scraper de producción y desarrollo siguen activos. Los recursos PostgreSQL antiguos también existen; Dokploy no muestra proveedor de copias de seguridad configurado para las bases observadas.
- No se creó un proyecto/servicio Dokploy para WalacTV Next ni se eligieron su rama de despliegue, base propia, red o secretos. La publicación de código no despliega nada mientras no exista ese servicio. No se ha desplegado Next ni transferido ningún dominio. Ningún recurso antiguo se modificó o eliminó.

## Destinos pendientes

| Elemento | Estado | Evidencia necesaria |
|---|---|---|
| Repositorio | Público, `origin` configurado y remoto vacío en la última consulta | Verificar remoto/refspec y destinos automáticos de la rama justo antes de publicar |
| Rama de Dokploy, proyecto y servicio nuevos | Pendientes | Elegir el destino en el panel y no inferirlo de la rama local `main` |
| Auto Deploy nuevo | Pendiente | Confirmar el disparador y todos los entornos que recibirían la rama elegida |
| Web anterior | Revalidada en el panel el 2026-10-30 | Consultar el inventario privado para su dominio, ruta, repositorio, rama y última revisión exitosa antes del corte |
| PostgreSQL nueva | No creada ni elegida; las bases antiguas siguen presentes y no tienen proveedor de copias configurado | Elegir recurso separado, comprobar red interna, SSL, capacidad y procedimiento de backup/restauración |
| Dominio final | Elegido por el usuario: hostname actual de la web anterior | Verificar hostname/certificado en Dokploy al configurar Next; reasignarlo solo en el corte y mantener la web anterior disponible para recuperación |

El Compose productivo es `compose.dokploy.yml`, que extiende las definiciones de `compose.yml`. Usa la red `dokploy-network` (o el nombre confirmado mediante `DOKPLOY_NETWORK_NAME`) para permitir acceso interno a PostgreSQL y al proxy. `compose.local.yml` es solo para desarrollo y nunca se combina en Dokploy. Consulta el [runbook de publicación y recuperación](dokploy-release.md).

## Inventario antiguo y candidatos de retirada

Revalidado en la interfaz autenticada de Dokploy el 2026-10-30. La tabla indica actividad observada en contenedores y configuración, no métricas de tráfico ni prueba completa de consumidores. El inventario privado guarda rutas e identificadores exactos.

| Recurso | Evidencia actual | Evaluación |
|---|---|---|
| Web anterior de producción | Sigue ejecutándose; HTTPS y Auto Deploy `On Push` activos; última revisión exitosa disponible | Candidata a retirar únicamente tras desplegar Next, transferir la ruta, completar aceptación y ventana de recuperación |
| API IPTV de producción | API, Nginx y extractor ejecutándose; API y extractor saludables | Conservar: Android documenta el contrato de autenticación, catálogo, reproducción, favoritos y progreso |
| Scraper de producción | Nueve contenedores de trabajos/sincronización ejecutándose; se vio un despliegue fallido reciente, pero los contenedores actuales siguen activos | Conservar: alimenta datos usados por la API anterior; revisar fallos sin interrumpir trabajos |
| API IPTV y scraper de desarrollo | Ambos servicios y sus contenedores están ejecutándose | No declarar sin uso por falta de métricas; confirmar consumidores, tareas, volúmenes y responsables antes de retirar |
| PostgreSQL de producción y desarrollo | Ambos recursos existen; no se encontró proveedor de copias configurado | Conservar hasta verificar conexiones, datos, retención y restauración; no reutilizarlos para Next |
| Volúmenes y redes compartidos legacy | Las definiciones Compose locales muestran recursos compartidos por API/scraper | Conservar hasta comprobar montajes y consumidores reales en el host |

Las dos instancias de scraper y las dos API tienen Auto Deploy `On Push`. Una rama legacy puede afectar más de un servicio/entorno; no publicar ahí para preparar WalacTV Next. La actividad actual no autoriza cambios de configuración ni borrado. No se ha eliminado ningún elemento.

El proyecto/servicio nuevo propuesto es `WALACTV NEXT` / `WEB NEXT`, separado del `WEB` anterior, con PostgreSQL propia. El dominio final elegido ya está en uso por la web antigua; se mantiene allí durante la instalación. Validar salud de contenedores y conectividad interna antes de retirar la ruta antigua y asignarla a Next. `compose.dokploy.yml` mantiene los servicios en la red de aplicación y los une a la red compartida de Dokploy: `migrate`, `bootstrap-admin` y `api` necesitan acceder al PostgreSQL externo; `web` necesita acceder a `api` y al proxy. Verificar el nombre real de red antes de desplegar. API y base no publican puertos. La instalación local continúa usando `compose.yml` más `compose.local.yml` en una red aislada.

El despliegue mantiene `API_REPLICAS=1`; `VOD_TRANSCODING_ENABLED` queda en `false` hasta comprobar capacidad productiva. `DATABASE_URL`, `APP_ORIGIN`, `SESSION_SECRET`, `PROVIDER_IDENTITY_KEY`, `IPTV_ENCRYPTION_KEY`, `BOOTSTRAP_ADMIN_USERNAME`, `BOOTSTRAP_ADMIN_PASSWORD`, `IPTV_ALLOWED_HOSTS`, `TMDB_API_TOKEN` y `DOKPLOY_NETWORK_NAME` se configuran de manera privada en Dokploy. Registrar solo nombres/referencias. El nuevo sistema tiene usuarios y catálogo propios: no migra automáticamente cuentas, sesiones, fuentes, favoritos o progreso legacy.

## PostgreSQL y migraciones

- Desarrollo usa una base nueva exclusivamente local. No se importa ni modifica información de aplicaciones anteriores.
- Producción de Next usará PostgreSQL como recurso independiente de Dokploy, separado de las bases legacy existentes. Recurso, red, SSL, secretos y backup/restauración siguen pendientes.
- Las bases legacy de producción y desarrollo se observaron en el panel, pero no tenían proveedor de copias configurado. No se inspeccionaron datos ni valores de conexión; su relación con los servicios debe verificarse antes de cualquier retirada.
- `DATABASE_URL` es el nombre del secreto de conexión. Sus valores nunca se documentan ni imprimen con Compose resuelto.
- `compose.yml` usa la conexión externa definida por `DATABASE_URL`. En el entorno local, `compose.local.yml` la reemplaza por PostgreSQL 17 aislada, dentro del volumen `walactv-local-postgres-data`, sin publicar el puerto de base de datos al host.
- Compose ejecuta `walactv migrate` y después `walactv bootstrap-admin` como trabajos puntuales antes de iniciar la API. El bootstrap usa `BOOTSTRAP_ADMIN_USERNAME` y `BOOTSTRAP_ADMIN_PASSWORD`, no imprime sus valores y debe ser idempotente para permitir reinicios del stack.
- Una actualización/rollback de imagen no revierte el esquema. Se preferirán cambios aditivos; la restauración de backups requiere un procedimiento comprobado antes de producción.

## Configuración

Variables: `DATABASE_URL`, `APP_ORIGIN`, `SESSION_SECRET`, `PROVIDER_IDENTITY_KEY`, `IPTV_ENCRYPTION_KEY`, `IPTV_ALLOWED_HOSTS`, `TMDB_API_TOKEN` (opcional), `API_REPLICAS`, `BOOTSTRAP_ADMIN_USERNAME` y `BOOTSTRAP_ADMIN_PASSWORD`. `TMDB_API_TOKEN` se configura solo como secreto runtime del servicio API; `.env.example` solo incluye el nombre vacío, nunca un valor real. No llega a builds, bundles, respuestas ni logs. Si no está configurado, la metadata indica `disabled` y catálogo/playback funcionan igual. `API_REPLICAS` debe permanecer en `1` mientras los leases, streams e importaciones activas de IPTV estén en memoria. El override local añade `LOCAL_POSTGRES_DB`, `LOCAL_POSTGRES_USER` y `LOCAL_POSTGRES_PASSWORD`. `./scripts/setup-local.sh` crea `.env` ignorado con permisos 600, genera secretos locales y no muestra sus valores. Conserva un `.env` existente y no genera token TMDB. `IPTV_ALLOWED_HOSTS` queda vacío hasta que se añada manualmente el dominio proveedor a `.env`; no se incluye URL, usuario ni contraseña de IPTV. Las credenciales IPTV entran por POST y se cifran en Go antes de guardarse en PostgreSQL. `IPTV_ENCRYPTION_KEY` es base64 de exactamente 32 bytes, estable y externa a la base/Git; se pasa solo al proceso API, nunca a build args ni al navegador.

## Arranque local

Requisitos: Docker Compose v2 y OpenSSL. Desde la raíz del repositorio:

```sh
./scripts/setup-local.sh
docker compose -f compose.yml -f compose.local.yml up --build -d
docker compose -f compose.yml -f compose.local.yml ps
```

La web escucha solo en `127.0.0.1:8081`; Nginx entrega `/api/` a Go por la red interna y desactiva buffering en el proxy de medios. Los tiempos de lectura y envío en medios tienen un límite de inactividad de 60 segundos, no un límite total de duración. PostgreSQL y API no publican puertos al host. Consulta la contraseña bootstrap directamente desde el `.env` local; no la copies a documentación ni informes.

Para parar la aplicación y conservar datos: `docker compose -f compose.yml -f compose.local.yml down`. Para eliminar intencionalmente la base **local** y su volumen: añade `-v`. Esto no modifica ninguna base externa.

El Compose base omite PostgreSQL local y requiere `DATABASE_URL` del recurso PostgreSQL externo. El archivo `compose.dokploy.yml` es el punto de entrada productivo. No usar `compose.local.yml` en Dokploy. El nuevo destino, rama, Auto Deploy, PostgreSQL propia, red, SSL, backups y secretos productivos siguen pendientes de configuración y verificación. El dominio final está elegido y permanece en el servicio anterior hasta el corte.

## Comprobaciones

Tras implementar se verificarán los comandos Go, web, validación Compose sin salida de secretos y salud `/healthz`. Las pruebas de streams usan fuentes simuladas; las pruebas reales se harán de una en una, respetando el límite del proveedor. Registrar implementación, publicación y despliegue verificado por separado.

## Verificación local realizada · 2026-10-28

`docker compose -f compose.yml -f compose.local.yml up --build -d` crea el PostgreSQL independiente, ejecuta `walactv migrate`, `walactv bootstrap-admin` y después `walactv serve`. Los jobs terminaron con código 0; PostgreSQL/API/web saludables. Migradores concurrentes y bootstrap repetido comprobados. `nginx -t`, Compose `config --quiet` y `/api/v1/health` correctos. Ver `docs/local-verification.md` para evidencia y límites.

El repositorio público existe pero sigue sin commit ni push. No hay servicio Dokploy de Next ni despliegue productivo que verificar. Las ramas/Auto Deploy nuevos siguen pendientes de configurar. El dominio de la fuente IPTV se configura explícitamente en `IPTV_ALLOWED_HOSTS` del entorno privado; no publicar la configuración local.

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

`IPTV_ALLOWED_HOSTS` sigue autorizando los dominios de las fuentes y del plano de control. Los servidores públicos de medios derivados de una reproducción autorizada, incluidas redirecciones y referencias HLS, no necesitan añadirse individualmente. La API valida las direcciones resueltas y el puerto en cada nueva conexión, y limita las redirecciones. Esto amplía la confianza en los destinos públicos indicados por el proveedor. No requiere migraciones, servicios ni variables nuevos. Un rollback de imagen vuelve a exigir configurar los CDN explícitamente en `IPTV_ALLOWED_HOSTS`; la base de datos permanece compatible.

## Diagnóstico y compatibilidad de medios VOD

Las reproducciones autorizadas admiten hasta ocho redirecciones y puertos HTTP 80, 8000, 8080, 8081, 8880 y 8888 o HTTPS 443 y 8443. El control del proveedor conserva sus propias restricciones. La API expone al propietario un diagnóstico saneado del último fallo de reproducción, sin URLs ni credenciales y sin volver a solicitar el vídeo. La compatibilidad de contenedor y códecs depende del navegador. No hay migraciones, servicios ni variables nuevos. Al volver a una imagen anterior se recuperan las restricciones anteriores y el diagnóstico genérico, sin cambios de base de datos.

## Conversión de vídeo local para navegadores

`VOD_TRANSCODING_ENABLED` habilita el fallback opcional para películas y episodios que el navegador no consigue decodificar. El valor por defecto es `false`; FFmpeg se instala en la imagen de la API y ejecuta únicamente dentro de ese contenedor. La API transmite la entrada desde la fuente IPTV validada por un gateway loopback temporal, sin poner la URL privada en argumentos, logs o variables del proceso FFmpeg. La salida es HLS H.264/AAC hasta 720p, temporal y no persistente. Canales en directo y listas HLS no se convierten.

Compose limita el servicio API completo a 2 CPU, 1536 MiB y 128 procesos; la salida efímera usa un tmpfs de 256 MiB. Solo se permite un trabajo de conversión global, sin cola y conservando el lease IPTV existente. Vídeo 4K HEVC puede no convertirse en tiempo real dentro de ese presupuesto y se detiene tras los límites de preparación/inactividad. La búsqueda reinicia la conversión en el offset solicitado; la barra completa solo aparece con duración fiable. La salida HLS rueda aproximadamente 48 segundos; si se pausa más de 45 segundos, reanuda regenerando desde la posición lógica guardada. Al salir, cerrar sesión, eliminar la fuente o detener la API se cancelan procesos y se borra el trabajo.

En instalaciones locales, establecer `VOD_TRANSCODING_ENABLED=true` en `.env` y reconstruir Compose para habilitarlo. Producción requeriría evaluar CPU/RAM y habilitar la imagen/variable solo después de medir allí. No requiere migraciones de base de datos ni puertos/servicios/volúmenes persistentes nuevos. Un rollback de imagen cancela y elimina conversiones temporales; PostgreSQL no cambia.
