# Spec Delta

## Purpose

Proteger la aplicación nueva mediante acceso cerrado y conservar una biblioteca personal independiente de las credenciales privadas del proveedor IPTV.

## ADDED Requirements

### Requirement: Acceso cerrado y sesión
El sistema SHALL permitir bootstrap de administrador sin registro público, login y logout mediante cookie HttpOnly del mismo origen. SHALL validar Origin y CSRF en mutaciones protegidas y revocar medios al cerrar o vencer la sesión.

#### Scenario: Sesión válida y logout
- **WHEN** se inicia sesión con credenciales válidas
- **THEN** la UI recibe usuario y token CSRF sin exponer cookie a JavaScript
- **WHEN** se cierra sesión
- **THEN** nuevos accesos privados y medios quedan denegados

#### Scenario: Escritura cross-site
- **WHEN** falta CSRF válido o el origen no coincide
- **THEN** se rechaza la mutación sin modificar datos

### Requirement: Biblioteca privada persistente
El sistema SHALL guardar favoritos y progreso por usuario y contenido estable, aislados por cuenta proveedor y tipo. SHALL conservarlos al reiniciar sin persistir contraseñas del proveedor; progreso usa segundos finitos no negativos.

#### Scenario: Reintroducir una fuente
- **WHEN** se vuelve a registrar la misma cuenta tras reinicio
- **THEN** se recupera su biblioteca sin depender del ID temporal de fuente ni colisionar con otra cuenta de igual ID numérico

#### Scenario: Acceso de otro usuario
- **WHEN** B intenta leer o eliminar el favorito/progreso de A
- **THEN** no recibe ni modifica datos privados de A
