# WalacTV Next

@docs/deployment.md

- Proyecto nuevo autorizado por el usuario, 2026-10-28. API Go y web Angular standalone/TypeScript completamente nuevas; sin dependencia de la API Python anterior.
- Lee OpenSpec antes de implementar. El usuario autorizó iniciar la versión local desde cero; migraciones de usuarios antiguos y compatibilidad Android quedan para fases posteriores.
- SDD usa los roles personales sdd_planificador, sdd_implementador y sdd_revisor. No delegar de nuevo desde un subagente.
- Nunca guardar credenciales reales, URLs privadas con contraseña, .env, cookies ni claves en Git, logs, screenshots o fixtures. Las fuentes IPTV privadas se guardan cifradas en PostgreSQL por usuario mediante IPTV_ENCRYPTION_KEY estable; credenciales descifradas solo en memoria del API. Cerrar sesión revoca streams pero conserva la fuente; eliminar fuente borra persistencia y catálogo.
- Tema oscuro, interfaz en español, componentes independientes y rutas cargadas bajo demanda. Validación y autorización siempre en Go, además de la interfaz.
- No modificar ni conectar servicios o bases de datos de proyectos anteriores. La PostgreSQL local es independiente; PostgreSQL de producción será externa a Compose.
- Todo push/despliegue necesita autorización y destino real verificado. El nombre de la rama local no identifica un entorno de Dokploy.
- Conservar cambios de otros agentes. Pruebas deterministas con proveedores simulados; las comprobaciones reales del proveedor las coordina el agente principal.
