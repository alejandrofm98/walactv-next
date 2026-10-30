# Tasks

Cada tarea se limita a una sesión acotada y registra su evidencia en `docs/deployment.md` o el runbook que este enlace. Las tareas de panel/publicación quedan pendientes hasta disponer de sus datos; la autorización existente se evalúa por acción y destino.

## 1. Preparación local

- [x] 1.1 Actualizar `docs/deployment.md` con el commit público inicial, el proyecto/servicio Dokploy creado, el dominio final elegido y los recursos/secrets/despliegue que siguen pendientes; mantener los detalles privados en el inventario local.
- [x] 1.2 Preparar `compose.dokploy.yml` y mapa de variables por nombre/referencia privada: conectar migrate/bootstrap-admin/api a la red compartida seleccionada, conservar web→api y proxy→web, solo web pública, una réplica y conversión desactivada inicialmente; validar la combinación productiva y la local con `config --quiet` y redes por servicio, usando configuración de ejemplo sin conectar a base real.
- [x] 1.3 Revisar los 120 archivos no ignorados: el escaneo saneado no encontró claves, credenciales embebidas, rutas personales, hostname de producción ni URL IPTV real. Confirmar que `.env` y `artifacts/` están ignorados. Antes del primer push, `origin` era el repositorio público nuevo sin referencias ni refspec, y no existía un servicio Dokploy de Next asociado; `main` se seleccionó para inicializar GitHub, no por inferencia del destino Dokploy.
- [x] 1.4 Redactar `docs/dokploy-release.md` para instalación, migración/bootstrap, dominio final, recuperación y continuidad de usuarios; verificar que comandos/rutas existen y que el runbook no promete migración legacy ni rollback automático de esquema.
- [x] 1.5 Redactar la matriz inicial de retirada de la web anterior, API IPTV, scraper, servicios de desarrollo y datos; no clasifica como eliminable ningún elemento con uso o datos pendientes.
- [x] 1.6 Validar `openspec validate prepare-dokploy-cutover --strict` y Compose `config --quiet`; registrar resultados locales y bloqueos externos separados de publicación/despliegue.

## 2. Revalidación del panel después del login

- [x] 2.1 Leer la configuración actual de WEB y recursos asociados; registrar dominio/puerto, repositorio/rama/Compose, Auto Deploy, commit/imagen recuperables y fuente/fecha en el inventario privado, con resumen público saneado.
- [ ] 2.2 Completar el proyecto/servicio ya creado `WALACTV NEXT` / `WEB NEXT`: repositorio `walactv-next`, rama `main`, Compose `compose.dokploy.yml`, Auto Deploy apagado; crear DB, configurar red/secrets y luego decidir Auto Deploy. El dominio final elegido es el hostname actual de WEB; verificar destinos antes de nuevos pushes.
- [ ] 2.3 Confirmar PostgreSQL nueva, redes reales/proxy, SSL, referencias de secretos, backup/restore y capacidad del servidor; verificar aislamiento frente a bases legacy, el acceso de los tres servicios de base y conservar `API_REPLICAS=1` antes de configurar recursos.
- [x] 2.4 Revalidar actividad actual de IPTV/SCRAPPER/dev y contrastar consumidores documentados, Compose, datos y recursos; completar la matriz sin inferir tráfico o responsables no observados. Todos siguen activos y no hay candidato de retirada demostrado.

## 3. Primera publicación separada

- [ ] 3.1 Tras verificar autorización y destino, publicar el commit seleccionado y configurar el servicio nuevo con Compose/secretos confirmados; registrar SHA, rama, configuración y resultado de Dokploy sin tratar el push como salud.
- [ ] 3.2 Ejecutar la primera instalación en PostgreSQL propia, comprobar `migrate` y `bootstrap-admin` con código 0 y versión de esquema, y ensayar recuperación en destino aislado; registrar evidencia sin secretos ni cambios a bases antiguas.
- [ ] 3.3 Tras validar salud interna y preparar recuperación, transferir el dominio final seleccionado y comprobar HTTPS, login, fuente, catálogo, reproducción secuencial y liberación; registrar resultados/límites y devolver la ruta anterior si falla aceptación.

## 4. Corte y recuperación

- [ ] 4.1 Definir responsable, ventana/criterios de recuperación y referencias de WEB anterior; verificar disponibilidad de su imagen/configuración y documentación del cambio de cuentas antes de transferir la ruta.
- [ ] 4.2 Con destino preparado y autorización aplicable, alinear origen final y transferir el dominio; verificar públicamente commit, HTTPS, salud y recorrido de aceptación, manteniendo WEB recuperable durante la ventana.
- [ ] 4.3 Verificar disponibilidad del procedimiento de recuperación y, si falla la aceptación, devolver la ruta a WEB anterior y comprobar salud; registrar el resultado o que no fue necesario ejecutarlo, conservando base/clave de Next sin migraciones down ni borrados.
- [ ] 4.4 Al terminar la ventana, entregar candidatos y condiciones concretas de retirada con consumidores/datos comprobados; verificar que ninguna eliminación permanente se ejecutó como parte de la preparación o corte.

## Workflow follow-up

- Una eliminación posterior de servicios o datos requiere su operación concreta, autorización aplicable, dependencias resueltas y recuperación/retención acordadas.
- Archivar este cambio después de completar las tareas del alcance acordado y su evidencia; no marcar publicación o corte como realizados por haber preparado documentos.
