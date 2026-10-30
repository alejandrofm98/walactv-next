## 1. Diagnóstico de API

- [x] 1.1 Clasificar errores de medios sin serializar secretos y conservar estado upstream numérico cuando existe.
- [x] 1.2 Asociar último fallo al playback autorizado con revocación/expiración y semántica clara para reintentos; conservar contrato anterior mediante campos opcionales o consulta privada.
- [x] 1.3 Añadir pruebas deterministas de clasificación, aislamiento y redacción de URLs, tokens y cuerpos upstream.

## 2. Compatibilidad de transporte

- [x] 2.1 Aplicar puertos HTTP 80/8000/8080/8081/8880/8888 y HTTPS 443/8443 y máximo ocho redirects exclusivamente a medios, incluidos HLS y dial.
- [x] 2.2 Probar octavo/noveno salto, puertos válidos/inválidos, control intacto y regresiones SSRF/TLS/cabeceras/Range/cancelación.

## 3. Reproductor

- [x] 3.1 Mostrar diagnóstico de transporte disponible y diferenciar límite 409, errores de navegador y causa desconocida, sin atribuir códecs sin evidencia.
- [x] 3.2 Confirmar cierre al salir/reintentar y evitar peticiones adicionales al proveedor para diagnosticar; probar mensajes con errores simulados.

## 4. Verificación y documentación

- [x] 4.1 Ejecutar comprobaciones Go/web afectadas, OpenSpec estricto, Compose config sin secretos y salud local; registrar resultado.
- [x] 4.2 Coordinador: comprobar secuencialmente tres películas y tres episodios de dos series, MP4/MKV si disponibles; cerrar cada reproducción antes de la siguiente y registrar avance real/seek, causas residuales y límites.
- [x] 4.3 Si llegan bytes pero no hay reproducción, investigar contenedor/códecs con muestra acotada y proponer alcance posterior si necesita conversión; no afirmar compatibilidad completa.
- [x] 4.4 Actualizar docs de política de medios, diagnóstico y rollback, explicitando que no se requieren migraciones ni variables/servicios Compose nuevos y que no se ha publicado.
