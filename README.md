# WalacTV Next

Aplicación nueva con web Angular y API Go. La IPTV privada se introduce en la sesión de la aplicación; no se guarda en el repositorio ni se incluye en las imágenes.

## Arranque local

Requisitos: Docker Engine con Compose v2 y OpenSSL.

```sh
./scripts/setup-local.sh
```

El script crea `.env` con permisos privados, genera los secretos locales y conserva cualquier `.env` que ya exista. Abre el archivo localmente para consultar la contraseña del usuario inicial `admin`. `IPTV_ALLOWED_HOSTS` es opcional: vacío permite proveedores Xtream con direcciones públicas; una lista separada por comas restringe los hosts. No compartas ni pegues el contenido completo del archivo.

```sh
docker compose -f compose.yml -f compose.local.yml up --build -d
```

La aplicación estará en <http://127.0.0.1:8081>. Compose espera a que PostgreSQL esté saludable, ejecuta migraciones y crea el usuario inicial antes de arrancar la API. El usuario de inicio es `admin`; su contraseña aleatoria está en `.env` y no se imprime durante el arranque.

```sh
docker compose -f compose.yml -f compose.local.yml ps
docker compose -f compose.yml -f compose.local.yml down
```

`down` conserva el volumen PostgreSQL. Para borrar también los datos locales de esta instalación, ejecuta `docker compose -f compose.yml -f compose.local.yml down -v`; esto elimina la base local y no afecta a PostgreSQL externo.

La web se publica únicamente en loopback, en el puerto 8081. La API y PostgreSQL no publican puertos del host. Los vídeos pasan por el proxy sin buffering para conservar streaming y solicitudes Range.

## Configuración de producción

Para Dokploy usa `compose.dokploy.yml`, que conecta la aplicación a la red externa de Dokploy y a PostgreSQL. La instancia de base de datos vive fuera del ciclo de vida de Compose. Configura los secretos enumerados en `.env.example` mediante el gestor de secretos del entorno. No uses `compose.local.yml` en Dokploy. Revisa [docs/deployment.md](docs/deployment.md) y el [runbook de publicación](docs/dokploy-release.md) antes de configurar un destino.

## Arquitectura y límites de esta versión

- `apps/api`: API Go, sesiones y biblioteca en PostgreSQL, catálogo Xtream y proxy de vídeo autenticado.
- `apps/web`: Angular standalone con rutas independientes y HLS descargado al reproducir.
- La lista privada debe pertenecer a un proveedor compatible con Xtream (`get.php`). Una M3U arbitraria sin API Xtream no está implementada.
- La fuente privada se guarda cifrada en PostgreSQL y se recupera automáticamente al iniciar sesión o reiniciar. Cerrar sesión conserva la fuente; «Eliminar fuente guardada» la borra junto con su catálogo. Favoritos y progreso se conservan.
- `IPTV_ALLOWED_HOSTS` puede quedar vacío para permitir cualquier proveedor Xtream con direcciones públicas; una lista separada por comas restringe los hosts de alta y control. Alta y control admiten HTTP:80 y HTTPS:443. Durante una reproducción autorizada, el proxy sigue redirecciones y referencias HLS hacia destinos públicos sin inscribir cada servidor; valida cada conexión y bloquea redes privadas y puertos no permitidos. El proveedor puede dirigir medios a cualquier destino público permitido por estas reglas.
- HLS, MP4 y MKV se entregan en su formato original. La compatibilidad depende también del códec y del navegador. No se incluye transcodificación.
- Ace Stream abre la aplicación instalada en Windows/Linux o Android desde una pulsación. El navegador puede pedir confirmación; iPhone/iPad no ejecutan ese motor local.
- Una réplica de API: los leases y streams activos siguen en memoria. Redis/coordinación compartida serían necesarios antes de escalarlos. No se incluye Torrentio ni migración de usuarios anteriores.

Consulta [las comprobaciones locales](docs/local-verification.md) para distinguir lo probado de los pendientes de producción.

## Fuente guardada y clave de cifrado

PostgreSQL guarda una fuente por usuario con su URL original, usuario y contraseña cifrados en Go mediante AES-256-GCM. El navegador recibe solo metadatos; el formulario no vuelve a mostrar las credenciales. Se conserva únicamente el catálogo solicitado (categorías, canales, películas y episodios), sin URLs privadas de reproducción; su caché dura 60 segundos y después se refresca automáticamente con el proveedor. No se precarga todo el servicio.

`IPTV_ENCRYPTION_KEY` es base64 de 32 bytes aleatorios. `scripts/setup-local.sh` la genera **una vez** al crear una instalación nueva, junto con los demás secretos. Si `.env` ya existe, se conserva: una clave ausente se considera un error que requiere recuperar la original. Nunca se reemplaza automáticamente una clave perdida. El servidor recibe esta variable únicamente en ejecución, fuera de la base de datos y de las imágenes.

Conserva una copia privada recuperable de la clave en un gestor de secretos y un backup independiente de PostgreSQL. **El backup de la base no basta sin la clave original.** Cambiarla sin migrar los datos impide descifrar las fuentes; la rotación automática no está implementada. En una instalación anterior a esta función, se debe crear una clave inicial antes de guardar la primera fuente, sin reutilizar otras claves del proyecto.

La migración `002` añade tablas y conserva usuarios/favoritos/progreso. `walactv migrate` recorre las versiones pendientes bajo lock, antes de iniciar la API. Un rollback de imagen conserva las tablas: la imagen anterior solo ofrece su comportamiento antiguo; volver a una imagen compatible y conservar la clave permite recuperar las fuentes guardadas. No se ejecutan borrados de esquema automáticamente.

Resultados de esta ampliación: [verificación de fuentes guardadas](docs/source-persistence-verification.md).
