# Publicación y sustitución en Dokploy

Este runbook prepara una publicación aislada de WalacTV Next. No migra datos ni cuentas de la web anterior. Revalidación del panel en modo de solo lectura, 2026-11-01, Dokploy v0.30.10: el proyecto `WALACTV NEXT` tiene `DB NEXT` y `WEB NEXT` en `production`; `WEB NEXT` apunta a `main` y `./compose.dokploy.yml`, Auto Deploy está activado con trigger `On Push`, y el panel muestra exitoso el commit `a83c78d` seguido por `c243fc50`. El push solicitado actualizará el entorno de producción. No se consultaron valores de secretos ni se cambió configuración del panel. Los backups se configurarán justo antes de importar la lista IPTV real; no se añadirá esa lista antes.

## 1. Confirmar el destino

Usar el inventario privado local, revalidado en Dokploy el 2026-10-30, como referencia de la web antigua y de las ramas con Auto Deploy. El servicio nuevo ya está configurado explícitamente con `main`; antes de futuros pushes, comprobar refspec y todos los destinos que recibirían esa rama. Confirmar el estado recuperable de la web anterior justo antes del corte.

El proyecto separado `WALACTV NEXT` y el servicio Compose `WEB NEXT` ya existen. El servicio está configurado con el repositorio público, la rama `main` y `compose.dokploy.yml`; la revalidación actual confirma Auto Deploy activo. Cualquier push a `main` inicia un despliegue de producción. El dominio final fue elegido por el usuario; su referencia exacta permanece privada. No cambiar rutas ni configuración del dominio durante este push.

## 2. Preparar PostgreSQL y redes

- Crear la base como recurso **Database → PostgreSQL** de Dokploy, independiente de Compose. No reutilizar la base de la aplicación anterior ni la base local. El usuario introducirá la contraseña durante su creación.
- Confirmar versión, dirección interna, puerto, SSL requerido y red compartida. El recurso debe quedar accesible desde `migrate`, `bootstrap-admin` y `api` sin publicar PostgreSQL a Internet.
- El panel no tiene proveedor S3/R2 configurado. El usuario pospuso configurar backups hasta inmediatamente antes de importar su lista IPTV real. Se puede preparar una instancia vacía, pero no se cargará la lista real hasta tener destino y restauración comprobados, además de una copia recuperable de `IPTV_ENCRYPTION_KEY`.
- Configurar `DOKPLOY_NETWORK_NAME` con el nombre real de la red que comparte la base y el proxy de Dokploy. La configuración productiva usa por defecto `dokploy-network`, pero verificarlo en el panel/host antes del primer despliegue.
- Configurar el servicio Compose con el archivo `compose.dokploy.yml`. Este archivo extiende las definiciones de `compose.yml`; no añadir `compose.local.yml`.
- La red `default` conserva `web → api`; la red Dokploy conecta a PostgreSQL los tres procesos que la necesitan y permite al proxy alcanzar `web`. La API y PostgreSQL no requieren puertos publicados.
- La primera instalación se despliega sin mover la ruta del dominio final. Confirmar salud de contenedores y conectividad privada antes del corte. En el corte, quitar el dominio de `WEB` y asignarlo al servicio Compose `web` de Next, puerto `80`, con HTTPS. No enrutarlo a `api`, `migrate` ni `bootstrap-admin`. En Docker Compose, los cambios de dominio requieren un redeploy para aplicar las etiquetas de Traefik.

La combinación productiva puede validarse localmente sin conectarse a una base real:

```sh
docker compose --env-file .env.example -f compose.dokploy.yml config --quiet
```

El archivo `.env.example` solo sirve para validar el formato; no copiar sus valores al panel. Validar la instalación local exclusivamente con `compose.yml` y `compose.local.yml`.

## 3. Configurar el servicio nuevo

En Dokploy, el servicio está vinculado al repositorio público `alejandrofm98/walactv-next`, rama `main`, tipo Docker Compose y archivo `compose.dokploy.yml`. Auto Deploy está activo con trigger `On Push`, verificado el 2026-11-01; cualquier publicación en `main` alcanza producción. No se modificarán variables, secretos ni opciones de Dokploy como parte de este push.

El usuario creará y cargará valores nuevos solo en la configuración privada de Dokploy. Registrar los nombres, no los valores:

| Variable | Configuración |
|---|---|
| `DATABASE_URL` | URL interna de la PostgreSQL nueva con SSL según la topología confirmada |
| `APP_ORIGIN` | Origen HTTPS exacto del hostname final elegido por el usuario; el valor concreto se mantiene en configuración privada |
| `SESSION_SECRET` | Secreto aleatorio nuevo, de al menos 32 bytes |
| `PROVIDER_IDENTITY_KEY` | Clave aleatoria nueva, de al menos 32 bytes |
| `IPTV_ENCRYPTION_KEY` | Clave base64 nueva que codifique exactamente 32 bytes; guardar copia recuperable fuera de la base y Git |
| `BOOTSTRAP_ADMIN_USERNAME` | Nombre de la cuenta inicial de Next, máximo 120 bytes |
| `BOOTSTRAP_ADMIN_PASSWORD` | Contraseña inicial nueva de 12 a 72 bytes; recuperarla desde el almacén privado de Dokploy |
| `IPTV_ALLOWED_HOSTS` | Opcional: vacío permite cualquier dominio proveedor con IP pública; lista separada por comas restringe por host |
| `API_REPLICAS` | Mantener `1` |
| `VOD_TRANSCODING_ENABLED` | Mantener `false` hasta medir recursos del servidor |
| `TMDB_API_TOKEN` | Opcional; runtime de API únicamente |
| `DOKPLOY_NETWORK_NAME` | Nombre real de la red compartida confirmada; si no se configura, el Compose usa `dokploy-network` |

No reutilizar secretos locales ni antiguos. Nunca añadir `.env`, URL IPTV, credenciales, tokens o valores privados a Git, Compose, logs, capturas o informes. La fuente IPTV se añade después, desde la sesión del usuario, y se guarda cifrada en la base nueva. No existe importación automática de cuentas, sesiones, fuentes, favoritos ni progreso de la aplicación anterior.

## 4. Primera instalación y preparación del corte

Desplegar solo cuando se haya confirmado que el servicio apunta a la PostgreSQL nueva y que la red externa existe. El orden Compose es `migrate` → `bootstrap-admin` → API saludable → web saludable. Si falla un trabajo, detener la promoción y leer el error saneado; no iniciar un reset ni apuntar a una base legacy.

Antes de transferir el dominio antiguo, verificar en el host de Dokploy o mediante las herramientas del servicio:

1. Commit y rama servidos corresponden al repositorio y destino confirmados.
2. `migrate` y `bootstrap-admin` finalizan con código 0; la API informa salud y web informa salud en su contenedor aunque la ruta pública aún permanezca en el servicio anterior.
3. API y trabajos tienen conectividad interna con la PostgreSQL nueva; el proxy llega a web; API y base no publican puertos.
4. Iniciar sesión con la cuenta Next recién aprovisionada. Antes de introducir la lista privada, configurar un destino de backup y comprobar restauración junto con la custodia externa de `IPTV_ENCRYPTION_KEY`.
5. Solo después de esa comprobación, añadir la lista privada del usuario e importar el catálogo.
6. Probar búsqueda/filtros y una reproducción autorizada cada vez. Cerrar la reproducción al terminar para no abrir sesiones simultáneas con el proveedor.
7. Confirmar que el navegador, logs e informe no muestran secretos ni URL IPTV con credenciales.

Anotar commit, versión de esquema, resultados de salud y límites de la prueba en `docs/deployment.md`. Un estado “push completado” no es evidencia de despliegue ni de salud. La aceptación de navegador se hará justo después de transferir el dominio, manteniendo lista la reversión de ruta.

## 5. Cambiar al dominio final

Antes del corte, definir quién opera el cambio, criterios de aceptación y ventana de recuperación. Confirmar que la instancia `WEB` anterior y su ruta pueden restaurarse; conservar intactas su base y configuración.

El API acepta un único `APP_ORIGIN` exacto. Configurarlo de antemano con el origen HTTPS final. Cuando Next esté saludable internamente y los trabajos hayan terminado bien, retirar el dominio del `WEB` anterior y asignarlo al `web` de Next, puerto `80`; redeplegar para aplicar la ruta. Verificar HTTPS, login, catálogo y la aceptación acordada enseguida. Mantener la web anterior recuperable durante toda la ventana.

Si falla la aceptación del dominio final, devolver la ruta pública a la web anterior y comprobarlo. La vuelta de ruta no revierte esquema ni elimina la nueva PostgreSQL. Conservar la PostgreSQL de Next y `IPTV_ENCRYPTION_KEY` para investigación y recuperación; no ejecutar migraciones inversas ni `DROP`.

## 6. Evaluar retiradas antiguas

Solo después de la ventana de recuperación, revisar cada candidato con datos del panel actual: consumidores, peticiones recientes si están disponibles, tareas programadas, redes, volúmenes, bases y responsables.

- Web anterior: candidata después de confirmar el cambio, aceptación y recuperación.
- API IPTV y scraper: conservar mientras Android u otros consumidores los usen.
- Servicios de desarrollo: no se sabe si siguen activos; no retirar hasta comprobarlo.
- PostgreSQL, volúmenes y redes antiguas: no retirar mientras exista cualquier consumidor o dato sin copia/restauración demostrada.

Esta revisión produce recomendaciones. No borra nada; una retirada requiere una decisión posterior con el destino y los datos concretos identificados.
