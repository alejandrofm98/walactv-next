# Design

## Context

Leer `AGENTS.md` y `docs/deployment.md`; este último será la fuente del mapa de destinos y evidencia operativa. No duplicar sus futuros valores confirmados de rama/dominio en otros documentos.

Hechos comprobados localmente y en Dokploy al 2026-10-30:

- El repositorio público `alejandrofm98/walactv-next` se inicializó en `main`, commit `27cd12f5dc73759070cc710e77c360488bd713e8`. Ese push ocurrió antes de enlazar Dokploy; el destino inicial de la rama y el remoto se verificaron vacíos.
- El panel Dokploy v0.29.5 se abrió autenticado y se revisó en solo lectura. La web antigua sigue activa en producción, con HTTPS, Auto Deploy `On Push` y una última revisión exitosa recuperable. Las API IPTV y los scraper de producción y desarrollo también tienen contenedores activos.
- El usuario eligió como dominio final el hostname que ya sirve la web antigua. La ruta se conservará durante el despliegue inicial de Next y se reasignará al servicio `web` solo después de validar la salud interna; la web anterior permanecerá recuperable para volver a asignarle la ruta.
- Los recursos PostgreSQL legacy de producción y desarrollo existen; el panel no muestra un proveedor de copias de seguridad configurado. No se han leído secretos ni valores de conexión. Ramas, IDs, hostnames y referencias exactas de recuperación se mantienen en el inventario privado local.
- Ya existe el proyecto `WALACTV NEXT` y el servicio Compose `WEB NEXT`, enlazado a `main` y `compose.dokploy.yml`. Auto Deploy está apagado; no hay despliegue, PostgreSQL propia o secretos configurados todavía.
- El contexto local de Android declara que consume la API IPTV anterior; esa API consume datos producidos por un scraper. La aplicación nueva no usa estos servicios, pero esto no convierte sus consumidores anteriores en prescindibles.
- `compose.yml` contiene trabajos `migrate` y `bootstrap-admin`, API y web. `compose.local.yml` añade exclusivamente la PostgreSQL local. La nueva API conserva leases, importaciones y streams en memoria y exige una réplica.

La solicitud actual prepara el corte y evalúa retiradas. El usuario autorizó el proyecto separado y eligió el dominio final; el servicio queda aislado y sin Auto Deploy hasta completar secrets/base. No se corta la ruta ni se modifica un servicio legacy durante la instalación inicial.

## Goals / Non-Goals

**Goals:** publicar una versión identificable en infraestructura propia; probarla antes de transferir el dominio; conservar recuperación; determinar con evidencia qué servicios antiguos podrían retirarse.

**Non-Goals:** migrar contratos/usuarios Android o legacy, reutilizar su base, cambiar la lógica IPTV/códecs, prometer alta disponibilidad, borrar servicios/datos en la preparación o declarar éxito por un push.

## Decisions

### Fuente del destino y fases

`docs/deployment.md` distinguirá hechos locales comprobados, inventario histórico, datos pendientes y evidencia productiva. Por cada destino incluirá repositorio, rama real seleccionada en Dokploy, proyecto/entorno/servicio, Auto Deploy y disparador, ruta Compose, servicio/puerto del dominio y fecha/fuente de verificación. Un push comprobará también refspecs y todos los destinos de la rama.

Preparación local: auditar archivos publicables/exclusiones, preparar configuración y runbook, validar Compose sin imprimir secretos y validar OpenSpec. Acceso al panel: revalidar servicio anterior, dominio, consumidores, recursos, capacidad, red, backups y destino nuevo. Publicación: registrar commit, resultado de Dokploy y salud. Corte: transferir el dominio después de comprobar trabajos, salud de contenedores y conectividad interna. Estos estados se registran separados.

### Servicio separado antes del corte

Crear un proyecto Dokploy separado, propuesto `WALACTV NEXT`, y un servicio Compose independiente `WEB NEXT`. Proyecto/servicio, entorno, repositorio/rama y Auto Deploy aún no constan creados. Usar el Compose productivo documentado, sin `compose.local.yml`, y comprobar el aislamiento de nombres/proyecto/red/imágenes antes de ejecutarlo. Solo `web` por el puerto 80 necesita ruta pública; API y PostgreSQL permanecen en la red requerida para su comunicación.

El coordinador comprobó en la documentación oficial que la asignación de dominio Compose añade `dokploy-network` al servicio seleccionado; no acredita conectividad de los otros servicios con PostgreSQL. Preparar un override productivo o configuración equivalente que una `migrate`, `bootstrap-admin` y `api` a la red externa real del recurso PostgreSQL y conserve la red de aplicación para `web` → `api`. Confirmar también la red del proxy para `web`; no confiar en la inyección de dominio para conectar los trabajos de base. Registrar el nombre real confirmado del recurso/red. Mantener esta adaptación fuera del override local y validar por separado la combinación productiva y la local, sin publicar puertos de API o base.

El usuario seleccionó como dominio final el hostname que ya usa la web antigua; por tanto, no hay un hostname temporal separado confirmado. Desplegar Next mientras la ruta pública sigue en la web antigua. Validar migración, bootstrap, salud y conectividad interna. Configurar `APP_ORIGIN` con el dominio final y, cuando Next esté lista, retirar la ruta de la web antigua, asignarla al servicio `web` nuevo y hacer la aceptación desde el origen final. La configuración actual acepta un origen. No ampliar CORS para facilitar el corte.

### PostgreSQL y secretos propios

La PostgreSQL productiva nueva es un recurso independiente de Dokploy, separado del Compose y de la base legacy. Confirmar red, versión compatible, conexión, SSL según topología y backup/restore antes de usarla. No apuntar `DATABASE_URL` a las bases antiguas ni cargar datos locales o legacy como parte de esta preparación.

Preparar solo nombres/referencias de `DATABASE_URL`, `APP_ORIGIN`, `SESSION_SECRET`, `PROVIDER_IDENTITY_KEY`, `IPTV_ENCRYPTION_KEY`, `BOOTSTRAP_ADMIN_USERNAME`, `BOOTSTRAP_ADMIN_PASSWORD`, `IPTV_ALLOWED_HOSTS` y las opciones existentes. Generar secretos productivos propios a través del mecanismo privado elegido; no copiar `.env` local a Git ni a informes. Conservar `IPTV_ENCRYPTION_KEY` fuera de PostgreSQL con recuperación comprobada: un backup sin esa clave no recupera las fuentes cifradas. `TMDB_API_TOKEN` sigue opcional y solo runtime.

Mantener `API_REPLICAS=1`. `VOD_TRANSCODING_ENABLED` parte desactivado en producción; habilitarlo requiere comprobar capacidad y límites del servidor. Los límites Compose existentes (CPU, memoria, procesos y tmpfs) forman parte de la revisión, no una prueba de que el hardware disponible sea suficiente.

### Migración y compatibilidad

No se añade migración. La primera instalación sobre la base nueva ejecuta las migraciones pendientes 001–004 con la imagen del commit elegido. El trabajo `migrate` debe terminar con código 0 antes de bootstrap; `bootstrap-admin` debe terminar con código 0 antes de considerar lista la API. Un fallo conserva el servicio como no preparado y no habilita la ruta pública final.

Registrar versión de esquema y resultado de trabajos sin cadenas de conexión ni SQL con parámetros privados. Ensayar backup/restauración sobre un destino aislado, conservando la clave de cifrado; no usar la base productiva para pruebas o resets. Los cambios existentes son aditivos, pero volver de imagen no revierte esquema. La recuperación inicial del dominio usa el servicio viejo y su base intacta; no ejecuta `DROP`, migración down ni restaura automáticamente la base nueva.

La nueva instalación tiene autenticación y cuentas propias. Las sesiones JWT, usuarios, favoritos, progreso y fuentes del sistema anterior no se transfieren por compartir dominio. El runbook debe comunicar nueva entrada/provisión de usuarios y reintroducción de listas cuando corresponda, sin prometer migración no implementada.

### Promoción y recuperación

Antes del corte guardar referencias del servicio viejo: configuración de ruta/puerto, commit/imagen recuperables, estado y referencias privadas de configuración. Definir responsable y duración de la ventana de recuperación, además del criterio de aceptación; esos datos están pendientes, no se inventa un plazo.

Antes del corte, la publicación debe acreditar commit ejecutado, despliegue correcto, trabajos, `/healthz` web, `/api/v1/health` y conectividad a PostgreSQL propia. Después de transferir el dominio, verificar HTTPS, login/fuente/catálogo y una reproducción autorizada de muestra secuencial cuando exista cuenta disponible. Comprobar que el navegador no recibe secretos y que cerrar sesión/salir libera la reproducción. No se declara estabilidad prolongada por una muestra breve.

Una vez preparado y autorizado el corte, alinear origen final y ruta del dominio con `WEB NEXT`; verificar el dominio público y la versión. Conservar `WEB` recuperable durante la ventana acordada. Si fallan autenticación/API, rutas o reproducción necesaria, devolver la ruta al servicio anterior y registrar estado; conservar la base nueva y su clave para investigación/recuperación. Un rollback de ruta no es rollback de esquema.

### Evaluación de servicios antiguos

El informe de retirada incluirá servicio, consumidores, datos/volúmenes/redes, evidencia/fecha de uso, dependencia y condición para retirarlo. La clasificación inicial es:

| Elemento histórico | Condición inicial | Evidencia que falta para retirarlo |
|---|---|---|
| Web anterior | Candidata después del corte y ventana de recuperación | Configuración actual, versión recuperable, dominio transferido y aceptación del nuevo servicio |
| API IPTV anterior | Conservar: Android depende de su contrato | Confirmar migración/retirada de todos sus clientes y datos necesarios |
| Scraper anterior | Conservar: alimenta datos del API usado por Android | Confirmar que sus consumidores dejaron de necesitar sus trabajos/JSON |
| Servicios de desarrollo | Contenedores activos; consumidores y responsables no demostrados | Clientes, tareas de desarrollo, redes, volúmenes y responsables actuales |
| Bases, volúmenes y redes anteriores | Conservar; no había proveedor de backup configurado en las bases observadas | Dependencias actuales, datos retenidos y restauración comprobada |

La presencia de contenedores activos y las copias locales no prueban tráfico ni responsabilidad actual, pero sí descartan clasificar estos servicios como detenidos o evidentemente huérfanos. No retirar por nombre, antigüedad o porque Next no los use. La preparación produce candidatos y condiciones; un borrado irreversible es una operación posterior concreta con autorización aplicable y evidencia suficiente.

## Risks / Trade-offs

- Aunque el dominio final ya está elegido, los servicios/base/red nuevos siguen sin crear; no transferir la ruta hasta completar el despliegue y la salud interna.
- Las ramas antiguas pueden disparar varios entornos: no publicar en repositorios legacy para preparar Next.
- El corte cambia autenticación/datos de la web: preparar la nueva cuenta y comunicar la continuidad disponible.
- Las fuentes cifradas requieren su clave original: custodiarla con recuperación comprobada y separada de Git/base.
- La recuperación exige conservar servicio y datos viejos: la eliminación temprana elimina esa opción.

## Open Questions

Pendientes operativos que bloquean las fases productivas: PostgreSQL propia/red/SSL, credenciales y claves privadas, destino de backups, activar Auto Deploy cuando sea seguro, verificar certificado/ruta del dominio final, capacidad del servidor, cuenta de aceptación, ventana de recuperación y consumidores/responsables reales de servicios dev. Registrar la evidencia en `docs/deployment.md` antes de la tarea dependiente.

## Verification

Validar `compose.yml`, la combinación productiva añadida y la combinación con `compose.local.yml` con valores sintéticos mediante `config --quiet`; inspeccionar redes por servicio sin imprimir secretos y no levantar servicios ni conectar a PostgreSQL real para validar documentos. Revisar que los archivos publicables excluyen `.env`, backups y credenciales. Ejecutar `openspec validate prepare-dokploy-cutover --strict`. La verificación externa registra commit, trabajos, conectividad de base, resultado del despliegue y salud; cada fase conserva sus límites y tareas pendientes.
