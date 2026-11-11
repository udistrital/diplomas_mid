# diplomas_service

API MID para la gestión y generación de diplomas digitales de la Universidad Distrital Francisco José de Caldas.

## Especificaciones técnicas

- Go 1.25.
- Beego v2.3.10.
- Chromium para generar archivos PDF.
- Docker y Docker Compose para ejecución en contenedores.

## Variables de entorno

Configurar las variables definidas en [conf/app.conf](conf/app.conf) mediante el entorno o un archivo `.env` local.

```shell
DIPLOMAS_MID_HTTP_PORT=8090
DIPLOMAS_MID_RUNMODE=dev
```

También se requieren las URLs de los servicios consumidos y la configuración AWS/S3 para las operaciones correspondientes. No versionar credenciales.

## Ejecución del proyecto

```shell
git clone -b develop https://github.com/udistrital/diplomas_service.git
cd diplomas_service
# Configurar las variables de entorno antes de iniciar.
go run .
```

Para consumir la API desde un frontend local, usar modo `dev`. En producción, CORS permite los subdominios institucionales.

## Ejecución con Docker

El Dockerfile está preparado para CI/CD. Requiere el binario `main` para Linux amd64 y el directorio `runtime/` con Chromium y sus bibliotecas, generados por el paso de compilación de [.drone.yml](.drone.yml).

Con esos recursos preparados:

```shell
docker compose up --build
```

## Pruebas

```shell
go vet ./...
go test ./...
go build ./...
```
