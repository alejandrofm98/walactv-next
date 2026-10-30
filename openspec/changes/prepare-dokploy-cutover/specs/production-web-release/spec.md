# Spec Delta

## Purpose

Permitir publicar WalacTV Next y sustituir la web anterior mediante un destino verificado, datos independientes y recuperación del servicio público sin interrumpir a los consumidores legacy.

## ADDED Requirements

### Requirement: Destino de publicación verificable
La publicación SHALL usar un mapa en `docs/deployment.md` con repositorio, rama real de Dokploy, entorno/servicio, Auto Deploy, Compose, ruta pública y fuente/fecha. Los datos desconocidos SHALL figurar pendientes; la rama local o un inventario histórico SHALL NOT acreditar el destino actual.

#### Scenario: Panel sin acceso autenticado
- **WHEN** el panel presenta login y no permite verificar la configuración
- **THEN** continúa la preparación local y quedan pendientes las fases que requieren dominio, rama, recursos o configuración actuales

#### Scenario: Panel autenticado con destino nuevo aún sin configurar
- **WHEN** se puede leer el servicio anterior, pero no se ha elegido ni creado el destino de Next
- **THEN** se registra la referencia recuperable de la web anterior y se mantienen pendientes el servicio nuevo, su rama, dominio, red y PostgreSQL propia

#### Scenario: Push preparado
- **WHEN** se va a ejecutar un push autorizado
- **THEN** se comprueban remoto/refspec y rama destino reales, y se identifican los entornos afectados por su Auto Deploy

### Requirement: Instalación productiva independiente
Next SHALL publicarse en un servicio independiente antes del corte y usar PostgreSQL externa propia. SHALL conservar servicios y datos legacy mientras sus consumidores los necesiten. El override de PostgreSQL local SHALL NOT formar parte del despliegue productivo.

#### Scenario: Primera publicación paralela
- **WHEN** se prepara la instancia nueva antes de transferir el dominio final existente
- **THEN** usa proyecto, servicio y base propios, y el servicio público anterior continúa atendiendo sin cambiar su ruta, esquema o datos

### Requirement: Conectividad interna de aplicación y base
La configuración productiva SHALL conectar migrador, bootstrap y API a la red real de PostgreSQL externo, y web a API y al proxy del dominio. SHALL verificar cada enlace requerido sin depender de que asignar el dominio conecte todos los servicios. La configuración local SHALL conservar su red y base aisladas.

#### Scenario: Dominio asignado a web
- **WHEN** Dokploy conecta al proxy el servicio web seleccionado para el dominio
- **THEN** migrador, bootstrap y API siguen teniendo conectividad propia comprobada hacia PostgreSQL, y API/base no requieren puertos públicos

#### Scenario: Override productivo preparado
- **WHEN** se añade la configuración de redes productiva
- **THEN** se validan por separado las combinaciones productiva y local y la instalación local conserva su PostgreSQL independiente

### Requirement: Configuración privada recuperable
Los secretos productivos SHALL permanecer fuera de Git, bundles e informes. La clave que cifra fuentes SHALL conservarse con recuperación independiente de PostgreSQL. La configuración SHALL mantener una réplica y habilitar conversión VOD solo después de comprobar capacidad productiva.

#### Scenario: Recuperar fuentes guardadas
- **WHEN** se restaura un backup de la base nueva
- **THEN** existe acceso privado a la clave original y no se genera una sustituta que haga ilegibles las fuentes

### Requirement: Migraciones antes de declarar disponibilidad
La publicación SHALL aplicar las migraciones existentes pendientes y completar bootstrap antes de declarar lista la API. Un fallo SHALL impedir la promoción. El rollback de imagen o de ruta SHALL NOT ejecutar una reversión automática del esquema.

#### Scenario: Migrador fallido
- **WHEN** el trabajo de migración o bootstrap termina con error
- **THEN** la versión no se considera preparada y el dominio público anterior conserva su destino

#### Scenario: Instalación nueva correcta
- **WHEN** las migraciones y bootstrap terminan correctamente sobre PostgreSQL propia
- **THEN** se registran commit, versión de esquema y resultados saneados antes de comprobar disponibilidad y promover

### Requirement: Promoción con evidencia de ejecución
La versión SHALL superar comprobaciones internas antes de transferir el dominio final existente. El corte SHALL alinear origen de la aplicación y ruta pública, y acreditar commit ejecutado, despliegue y salud. Un push exitoso SHALL NOT equivaler a despliegue verificado.

#### Scenario: Reasignar el dominio final existente
- **WHEN** Next pasa las comprobaciones internas y se prepara la ruta final elegida por el usuario
- **THEN** se retira el hostname del servicio anterior, se asigna a Next, y se comprueban HTTPS, salud web/API y el recorrido de aceptación acordado con la versión anterior lista para recuperar la ruta

#### Scenario: Cambiar el dominio
- **WHEN** se ejecuta el corte autorizado hacia Next
- **THEN** el origen configurado coincide con el dominio final y la verificación pública identifica la versión y su salud

### Requirement: Recuperación conservando la web anterior
Antes del corte SHALL existir una referencia recuperable de la web anterior, su ruta y configuración, con responsable y ventana de recuperación definidos. Ante fallo de aceptación SHALL poder devolverse el dominio anterior sin borrar la base o clave de Next.

#### Scenario: Fallo después del corte
- **WHEN** la versión nueva no supera la aceptación del dominio público durante la ventana acordada
- **THEN** se devuelve la ruta al servicio anterior, se registra el resultado y se conservan los datos nuevos para recuperación

### Requirement: Continuidad de cuenta explícita
La sustitución SHALL comunicar que Next usa cuentas y sesiones propias. SHALL NOT prometer migración automática de usuarios, listas, favoritos o progreso del sistema anterior ni considerar válida su sesión por compartir el dominio.

#### Scenario: Primera entrada tras el cambio
- **WHEN** una persona accede al dominio que ahora sirve Next
- **THEN** utiliza una cuenta provisionada para Next y las instrucciones explican la continuidad de datos realmente disponible

### Requirement: Retirada basada en consumidores actuales
La evaluación SHALL identificar consumidores, datos y condición de retirada por servicio. SHALL conservar la API IPTV anterior y su scraper mientras Android u otros clientes los necesiten. La preparación SHALL producir candidatos y condiciones sin borrar servicios, bases, redes o volúmenes.

#### Scenario: Servicio usado por Android
- **WHEN** el contexto o la verificación actual acredita un consumidor del backend anterior
- **THEN** ese backend y sus productores de datos se conservan hasta resolver la dependencia

#### Scenario: Candidato sin uso comprobado
- **WHEN** un servicio antiguo o dev no tiene evidencia suficiente de sus consumidores o datos actuales
- **THEN** se registra uso pendiente de comprobar y no se trata como eliminable por su nombre o antigüedad
