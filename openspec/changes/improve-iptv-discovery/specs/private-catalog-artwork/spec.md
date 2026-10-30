# Spec Delta

## Purpose

Mostrar logos y carátulas disponibles del proveedor mediante recursos autorizados, protegiendo secretos y evitando que contenido externo comprometa el servidor.

## ADDED Requirements

### Requirement: Imágenes privadas por contenido
El sistema SHALL extraer logos/carátulas disponibles del proveedor, guardar sus URLs exclusivamente cifradas y entregar al navegador solo rutas relativas autenticadas por fuente/tipo/ID. SHALL no ofrecer proxy de URL arbitraria ni exponer credenciales en respuestas, registros o fixtures.

#### Scenario: URL de imagen con datos privados
- **WHEN** un proveedor entrega stream_icon/cover con URL que contiene credenciales
- **THEN** PostgreSQL conserva únicamente ciphertext de esa URL y el frontend recibe un endpoint propio sin datos privados

#### Scenario: Fuente ajena y caché
- **WHEN** B solicita una imagen de A que ya está en caché
- **THEN** la autorización anterior a caché deniega acceso con 404

### Requirement: Descarga de imagen acotada y segura
El sistema SHALL validar HTTP(S), DNS público y cada conexión/redirección, bloquear destinos privados/metadata, limitar tiempo/tamaño/redirecciones y aceptar solo JPEG/PNG/WebP/GIF con firma válida. SHALL no reenviar cookies/autorización del cliente ni aceptar SVG/HTML como imagen.

#### Scenario: Redirección peligrosa o payload inválido
- **WHEN** la imagen redirige a IP privada, cambia DNS a destino privado, supera límites o devuelve HTML
- **THEN** se rechaza sin devolver payload ni URL original y la UI muestra fallback

### Requirement: Arte progresivo y fallback
La web SHALL mostrar logos en canales y carátulas en VOD cuando estén disponibles, reservar espacio y cargar imágenes bajo demanda. Ausencia o error SHALL producir fallback local accesible sin bucle de reintentos y sin impedir reproducción.

#### Scenario: Icono disponible y ausente
- **WHEN** se muestran un canal con logo válido y otro sin imagen
- **THEN** se ve el primero y un fallback local en el segundo, manteniendo títulos y acciones accesibles
