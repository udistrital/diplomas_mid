# diplomas_service

API MID para la gestion de diplomas digitales.

## Flujo implementado

`POST /v1/documento_digital/:id/firmar`

Orquesta el primer flujo de firma de diploma:

1. Valida el rol activo del firmante en `administrativa_amazon_api`.
2. Envia el PDF original a `firma_electronica_mid` por `POST /api/v2/firma_electronica`.
3. Recibe `file` firmado y `uuid_documento`.
4. Guarda el PDF firmado en S3 con la ruta `diplomas/{uuid_documento}/diploma.pdf`.
5. Actualiza `diplomas_crud` con `PUT /v1/documento_digital/:id/uuid`.

`GET /v1/firmantes/rol-activo?documento_identidad=10000000`

Valida si un documento pertenece a un secretario academico, decano, secretaria general o rector activo.

`POST /v1/firmantes/firma`

Valida el rol activo del firmante, sube la imagen de firma a `DOCUMENTOS_CRUD` y registra el enlace en `diplomas_crud.firma_firmante`.
La imagen recibida se normaliza antes de almacenar: se quita fondo claro, se convierte el trazo a negro, se recortan bordes vacios y se guarda como PNG transparente.

## Variables

```shell
DIPLOMAS_MID_HTTP_PORT=8090
FIRMA_ELECTRONICA_MID_URL=http://localhost:8080/api
DIPLOMAS_CRUD_URL=http://localhost:8081
ADMINISTRATIVA_AMAZON_API_URL=${ADMINISTRATIVA_AMAZON_API_URL}
DOCUMENTOS_CRUD_URL=${DOCUMENTOS_CRUD_URL}
AWS_REGION=us-east-1
AWS_ENDPOINT_URL=http://localhost:4566
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
DIPLOMAS_S3_BUCKET=diplomas-digital-local
```

Si corre en Docker y consume servicios del host, usar `host.docker.internal` en las URLs.

## Ejecucion

```shell
go mod tidy
go run .
```

Docker (preparar primero `main` y `runtime/` según sección CI/CD):

```shell
docker compose up -d --build
```

## CI/CD con Drone

Configuración basada en [lineamientos OAS](https://github.com/udistrital/lineamientos_oas/tree/master/generacion_de_apis/cicd).
El módulo Go y las variables `DIPLOMAS_MID_*` conservan sus nombres por compatibilidad.

El pipeline `.drone.yml` verifica formato, ejecuta `go vet`, pruebas con cobertura y compila
un binario Linux amd64 estático. Sonar analiza código y cobertura en eventos push;
las solicitudes de cambio ejecutan validación y compilación sin publicar ni usar secretos.
Se admiten `feat/*`, `feature/*`, `develop`, `release/*`, `hotfix/*` y `master`.

| Rama (push) | Imagen ECR | Servicio ECS | Clúster |
| --- | --- | --- | --- |
| `release/*` | `diplomas_service:release` y SHA corto | `diplomas_service_test` | `test` |
| `master` | `diplomas_service:latest` y SHA corto | `diplomas_service_prod` | `oas` |

El nombre real se obtiene de `DRONE_REPO_NAME`. Los pasos son secuenciales: un fallo
impide publicar o desplegar. ECS fuerza actualización y espera estabilidad del servicio.
`Dockerfile` empaqueta el binario compilado por Drone sobre distroless Debian 12,
siguiendo la estructura de [syllabus_mid](https://github.com/udistrital/syllabus_mid).
El paso build instala Chromium en Debian 12 y ejecuta `scripts/prepare-chromium-runtime.sh`
para copiar navegador, bibliotecas y configuración de fuentes a `runtime/`.
La imagen incluye estos archivos y `assets/` para generar diplomas sin instalar paquetes
en la imagen final ni requerir shell.

### Configuración externa requerida

1. Activar repositorio en Drone con ejecutor Docker Linux amd64.
2. Registrar secretos `SONAR_HOST`, `SONAR_TOKEN`, `AWS_ACCESS_KEY_ID`,
   `AWS_SECRET_ACCESS_KEY` y `AWS_REGION`. No habilitarlos para PR externos.
3. Crear/configurar proyecto SonarQube `diplomas_service`.
4. Proveer permisos AWS para publicar/crear repositorio ECR, actualizar servicios ECS
   y consultar su estado. Repositorio ECR usa nombre del repositorio GitHub.
5. Crear previamente clústeres, servicios y definiciones de tarea ECS indicados arriba.
   La tarea test debe consumir etiqueta `release`; producción, `latest`.
   Configurar arquitectura X86_64, puerto y health check adecuados al servicio.
6. Inyectar variables de `conf/app.conf` mediante definición de tarea o secretos ECS.
   Definir `DIPLOMAS_MID_HTTP_PORT=8090`, `DIPLOMAS_MID_RUNMODE=prod`, URLs de APIs,
   `AWS_REGION` y `DIPLOMAS_S3_BUCKET`. `AWS_ENDPOINT_URL` se reserva para emuladores
   o endpoints alternativos. En ECS preferir rol de tarea con acceso S3 a claves estáticas.
   Los secretos de publicación Drone no se transfieren automáticamente al contenedor.

Este pipeline actualiza servicios existentes; no crea infraestructura ni registra nuevas
versiones de definición de tarea. Cambios de variables, recursos o puerto requieren
actualizar dicha definición en AWS. La estabilidad ECS depende del health check configurado.

### Verificación local

```shell
go vet ./...
go test -coverprofile=coverage.out ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main .
# Ejecutar preparación en Debian 12 con Chromium instalado:
sh scripts/prepare-chromium-runtime.sh
docker build -t diplomas_service:ci .
```

Las pruebas que requieren `PDFA_TEST_INPUT` se omiten salvo configuración explícita.
Antes de `docker compose up -d --build`, preparar `main` y `runtime/` como en Drone.
La compilación y preparación deben usar Linux amd64 y Debian 12 para mantener compatibilidad
con las bibliotecas de la imagen final. `runtime/` contiene artefactos generados y no se versiona.

## Aprobados por período académico (MID)

El microfrontend consulta Parámetros mediante este servicio y exige selección explícita de un ID; no existe un período predeterminado. Se incluyen períodos PA activos e históricos, ordenados por Year y Ciclo numérico descendentes. La identidad es `Id`, aunque los nombres se repitan.

Configurar `PAZ_Y_SALVOS_MID_URL`, `PARAMETROS_CRUD_URL` y `OIKOS_API_URL` por entorno. La base MID debe incluir el prefijo de despliegue, si existe (por ejemplo `/paz_y_salvos_mid`); el cliente agrega `/v1/solicitud-grado/paz-salvos/aprobados`, sin duplicar `/v1` si ya viene en configuración. Hosts y prefijos de despliegue se configuran únicamente por entorno. Parámetros admite base con o sin `/v1`; Oikos utiliza `/v2/dependencia_padre/`. Docker Compose transmite las tres variables. No se usa `PAZ_Y_SALVOS_CRUD_URL` ni un `PERIODO_ID` global.

| Operación | Contrato |
| --- | --- |
| `GET /v1/diplomas/periodos-academicos` | Arreglo de `{Id, Nombre, Year, Ciclo, CodigoAbreviacion, Activo}` |
| `GET /v1/diplomas/estudiantes-aprobados?periodo_id=77` | Contrato previo de facultades/estudiantes y errores; mantiene filtros y exclusión predeterminada de registrados |
| `POST /v1/diplomas/estudiantes-aprobados/documentos` | JSON previo más `periodo_id` obligatorio; ejemplo `{"periodo_id":77,"dry_run":true}` |
| `GET /v1/ceremonias?documento_identidad_secretario=...&periodo_id=77` | Tablero; aprobados y pendientes del período elegido |
| `GET /v1/ceremonias/programas/275/estudiantes?documento_identidad_secretario=...&periodo_id=77` | Estudiantes del programa Oikos elegido |
| `POST /v1/ceremonias/{id}/asignar` | JSON previo más `periodo_id`; ejemplo `{"periodo_id":77,"documento_identidad_secretario":123,"programas":[{"programa_academico_id":275}]}` |

Ejemplo de respuesta del catálogo: `[{"Id":77,"Nombre":"2026-3","Year":2026,"Ciclo":"3","CodigoAbreviacion":"PA","Activo":true}]`. El ID 77 es solo ilustrativo: se valida contra el catálogo real en cada consulta MID. Para ese registro, documentos guardan `periodo_id=77`, `vigencia=2026`; `PerInsGrado=3` conserva el ciclo. La solicitud MID conserva `ProgramaAcademicoId=26` como `programa_academico_mid_id`, mientras `DependenciaOikosId=275` alimenta programa y `PadreId.Id=14` alimenta facultad. `codigo_estudiante_texto` conserva el código externo junto al campo numérico heredado.

MID se pagina con `limit=20` y offset hasta Total, con timeout global y contexto. Totales variables, páginas vacías prematuras, duplicados, período cruzado y fallos HTTP/envelope fallan sin entregar un lote parcial. Oikos se resuelve una vez por programa en cada lote; exige relación e hija/padre activos, padre único y facultad identificable por nombre institucional `FACULTAD ...`; recorre padres intermedios con detección de ciclos. Una resolución no inequívoca detiene el lote, identifica el estudiante y evita persistir facultad cero. Los códigos incompatibles con el contrato numérico actual del CRUD (incluidos ceros iniciales) producen error explícito, nunca conversión silenciosa.

La caché de aprobados dura cinco minutos y usa `(periodo_id, facultad_id)`. Las ceremonias/asignaciones ya persistidas y las operaciones de firmas mantienen su alcance previo; elegir un período no elimina asignaciones. La exclusión de programas ya asignados se aplica por programa y período; no cruza períodos.

Se conserva `services/env.go`: cargar `.env` después de inicializar Beego requiere sincronizar AppConfig, HTTPPort y RunMode. El servicio institucional hermano `academica_core_service` también carga dotenv y sincroniza configuración al iniciar. Eliminar el cargador sin reemplazo perdería ese comportamiento local; se mantienen prioridad del entorno, ausencia opcional de `.env` y pruebas de sincronización. No se agrega una dependencia dotenv ni se modifica el `.env` local.

Validación local: `go test ./...`, `go vet ./...`, `go build ./...`, `dev-validate` y `docker compose config --quiet` (este último incluido en dev-validate). En entornos con caché Go de solo lectura usar una ruta externa, por ejemplo `GOCACHE=/tmp/diplomas-go-cache`; httptest necesita puertos locales. No se requieren llamadas a servicios institucionales para ejecutar las pruebas.

### CORS para desarrollo integrado

Configurar `DIPLOMAS_CORS_ALLOWED_ORIGINS` como lista de orígenes separados por coma. El `.env` local permite `http://localhost:4200,http://localhost:4227`; cada despliegue define sus propios orígenes. El filtro Beego atiende preflight `OPTIONS` antes del router y permite explícitamente `Authorization` y `Content-Type`. Sin orígenes configurados, CORS permanece deshabilitado. Esto también aplica al acceder por el proxy local del MF: el navegador envía preflight cuando usa Bearer desde otro puerto.


El listado `GET /v1/ceremonias?documento_identidad_secretario=...` admite ausencia de `periodo_id` para devolver ceremonias y asignaciones persistidas sin consultar aprobados. Al enviar `periodo_id`, agrega programas aprobados/pendientes de ese período. Consultar estudiantes aprobados y asignar programas sigue requiriendo un período explícito. Crear y editar una ceremonia requieren periodo_id explícito; el listado sin período permite revisar y configurar registros históricos.


## Ceremonias configuradas por período

`POST /v1/ceremonias` y `PUT /v1/ceremonias/{id}` requieren `periodo_id` válido en Parámetros PA. Se persiste en `ceremonia_grado` y aparece en creación, listado, edición y detalle. Ejemplo de campos obligatorios: `{"periodo_id":77,"fecha_ceremonia":"2026-10-28","lugar":"Auditorio","cantidad_estudiantes_programada":5,"documento_identidad_secretario":123}`. El ID es ilustrativo; no existe período predeterminado.

El selector está en la configuración de ceremonias. Elegir período filtra ceremonias, aprobados y asignaciones; las ceremonias ofrecidas para asignar un proyecto deben tener ese mismo período. Un estudiante se consulta en MID para el período configurado, su documento conserva ese ID y no puede asignarse a una ceremonia de otro período. Programa Oikos y período son dimensiones independientes. Que un programa aparezca en dos períodos no permite copiar estudiantes entre ellos: se exige aprobación MID independiente en cada período.

Se valida duplicidad por estudiante/período, además del control de programa/período. Cambiar período de una ceremonia con estudiantes asignados se rechaza. El CRUD agrega validaciones e índice único activo para cubrir solicitudes concurrentes; la revisión previa en MID no sustituye esa garantía de base de datos.

**Activación necesaria:** aplicar `diplomas_crud/database/migrations/20261028_000001_ceremonia_periodo.go`, actualizar CRUD y después reiniciar este servicio/publicar MF. La migración quedó probada en PostgreSQL desechable; no se ejecutó contra bases institucionales. El servicio comprueba que CRUD devuelva el período persistido y rechaza una implementación antigua que ignore el campo.

Las ceremonias históricas conservan `periodo_id=NULL`: se muestran como “Sin configurar” al listar todos los períodos y no admiten nuevas asignaciones. Una ceremonia histórica sin estudiantes puede configurarse explícitamente desde edición; si ya tiene asignaciones requiere conciliación previa, sin deducir IDs por nombre/fecha ni reclasificar estudiantes automáticamente.


Los consecutivos del diploma, facultad, libro, folio y registro se reservan al asignar el estudiante a una ceremonia, utilizando su fecha y la parametrización de facultad/vigencia. Si falta la parametrización o falla la reserva, no se crea esa vinculación. Los reintentos reutilizan el diploma y registro existentes. Las asignaciones anteriores sin registro lo obtienen antes de registrar la primera firma. Una vez generado el diploma, se mantienen los bloqueos de edición y retiro del flujo.

El secretario usa POST /v1/documento_digital/:id/diploma/preview con documento_identidad_secretario para preparar también diplomas vinculados antes de la reserva temprana. Valida la facultad del secretario, reutiliza registros existentes y luego devuelve el PDF. GET conserva la consulta sin escrituras.
