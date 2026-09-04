# diplomas_mid

API MID para la gestion de diplomas digitales.

## Flujo implementado

`POST /v1/documento_digital/:id/firmar`

Orquesta el primer flujo de firma de diploma:

1. Valida el rol activo del firmante en `administrativa_amazon_api`.
2. Envia el PDF original a `firma_electronica_mid` por `POST /api/v2/firma_electronica`.
3. Recibe `file` firmado y `uuid_documento`.
4. Guarda el PDF firmado en S3 con la ruta `diplomas/{uuid_documento}/diploma.pdf`.
5. Actualiza `diplomas_crud` con `PUT /v1/documento_digital/:id/uuid`.

`GET /v1/firmantes/rol-activo?documento_identidad=51768982`

Valida si un documento pertenece a un secretario academico, decano, secretaria general o rector activo.

`POST /v1/firmantes/firma`

Valida el rol activo del firmante, sube la imagen de firma a `DOCUMENTOS_CRUD` y registra el enlace en `diplomas_crud.firma_firmante`.
La imagen recibida se normaliza antes de almacenar: se quita fondo claro, se convierte el trazo a negro, se recortan bordes vacios y se guarda como PNG transparente.

## Variables

```shell
DIPLOMAS_MID_HTTP_PORT=8090
FIRMA_ELECTRONICA_MID_URL=http://localhost:8080/api
DIPLOMAS_CRUD_URL=http://localhost:8081/v1
ADMINISTRATIVA_AMAZON_API_URL=http://api.intranetoas.udistrital.edu.co:8104
DOCUMENTOS_CRUD_GESTOR_DOCUMENTAL_URL=http://api.intranetoas.udistrital.edu.co:8502/v1/gestor_documental
AWS_REGION=us-east-1
AWS_ENDPOINT_URL=http://localhost:4566
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
DIPLOMAS_S3_BUCKET=diplomas-digital-local
```

Si corre en Docker y consume servicios del host, usar `host.docker.internal` en las URLs.

## Payload

Firma de diploma:

```json
{
  "nombre": "Diploma digital",
  "descripcion": "Firma de diploma digital con QR",
  "documento_identidad_firmante": 51768982,
  "metadatos": {
    "tercero_id_estudiante": 123456,
    "programa_academico_id": 85,
    "periodo_id": 20261,
    "vigencia": 2026,
    "tipo_documento": "diploma"
  },
  "representantes": [],
  "file": "BASE64_DEL_PDF"
}
```

Imagen PNG de firma del firmante:

```json
{
  "documento_identidad_firmante": 51768982,
  "IdTipoDocumento": 38,
  "nombre": "firma_diploma",
  "metadatos": {
    "vigencia": "2026"
  },
  "descripcion": "firma diploma 2026",
  "file": "BASE64_DEL_PNG"
}
```

`file` tambien acepta data URI, por ejemplo `data:image/png;base64,...`.

## Respuesta

```json
{
  "firma_id": "uuid-firma",
  "repositorio_documental": "diplomas",
  "documento_id": 156103,
  "uuid_documento": "uuid-documento",
  "firmante": {
    "rol": "secretario_academico",
    "orden": 1,
    "cargo_id": 158,
    "cargo": "SECRETARIO ACADEMICO FACULTAD DE CIENCIAS Y EDUCACION",
    "nombre": "NOMBRE FIRMANTE",
    "documento_identidad": 51768982
  },
  "s3": {
    "bucket": "diplomas-digital-local",
    "key": "diplomas/uuid-documento/diploma.pdf",
    "uri": "s3://diplomas-digital-local/diplomas/uuid-documento/diploma.pdf"
  },
  "dynamodb": {
    "table": "firma_electronica",
    "firma_id": "uuid-firma",
    "sk": "repositorio_documental#diplomas#documento_id#156103"
  }
}
```

## Ejecucion

```shell
go mod tidy
go run .
```

Docker:

```shell
docker compose up -d --build
```
