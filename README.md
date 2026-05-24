# TP Final SDP - Cripto-Analisis Educativo

Sistema distribuido para dividir el espacio de busqueda de una contrasena cifrada entre multiples workers.

## Stack

- Go 1.25+
- gRPC
- PostgreSQL
- Nginx
- Podman Compose
- Prometheus + Grafana
- Markdown + Mermaid/PlantUML
- Go test + tests de integracion

## Componentes

- `api-gateway`: API HTTP para crear jobs y consultar estado.
- `scheduler`: coordina rangos, workers y cancelacion global.
- `worker`: consume rangos y prueba combinaciones.
- `postgres`: persistencia de jobs, resultados, rangos y metricas historicas.
- `prometheus`: recoleccion de metricas.
- `grafana`: visualizacion de metricas.
- `nginx`: balanceo hacia servicios HTTP.

## Primeros comandos

```bash
podman-compose up --build
```

El archivo de compose se mantiene como `docker-compose.yml` porque es el nombre
convencional que tambien usa `podman-compose`.

```bash
go test ./...
```

Para aumentar el rendimiento se puede escalar la cantidad de workers y ajustar
la concurrencia interna de cada worker con `WORKER_CONCURRENCY`.

```bash
podman-compose up --build --scale worker=4
```

## Demo minima

Crear un job para buscar la cadena `ab` dentro del charset `ab`. La API calcula
el SHA-256 de `password` y usa automaticamente su longitud:

```bash
curl -s -X POST http://localhost:8080/jobs \
  -H 'Content-Type: application/json' \
  -d '{
    "password": "ab",
    "charset": "ab",
    "chunk_size": 2
  }'
```

Consultar el estado del job:

```bash
curl -s http://localhost:8080/jobs/<job_id>
```

## Interfaz De Terminal

Con los servicios levantados, se puede usar una interfaz interactiva para ver
los puertos, ingresar la password, charset y chunk size, y seguir el resultado
del job. Primero dejar los servicios corriendo:

```bash
podman-compose up --build
```

En otra terminal, abrir la interfaz:

```bash
make compose-terminal
```

Tambien se puede ejecutar localmente contra `localhost`:

```bash
make terminal
```

Por defecto consulta `http://localhost:8080`. Para usar otra URL:

```bash
API_BASE_URL=http://localhost:8088 make terminal
```

Los rangos asignados a workers usan leases con vencimiento configurable mediante
`SCHEDULER_RANGE_LEASE_TTL`. Si un worker cae, el scheduler puede reasignar el
rango luego del vencimiento.

Prometheus recolecta metricas de la API y del scheduler. El scheduler expone
contadores de rangos asignados, completados, vencidos y jobs encontrados en
`http://localhost:9100/metrics`.

Adminer expone una interfaz web para revisar PostgreSQL en
`http://localhost:8081`. Usar sistema `PostgreSQL`, servidor `postgres`,
usuario `sdp`, password `sdp` y base `crypto_jobs`.

## Resiliencia Y Seguridad

El scheduler persiste jobs y rangos en PostgreSQL cuando `DATABASE_URL` esta
configurado. Al reiniciar, carga el estado persistido y puede continuar
asignando rangos pendientes o leases vencidos.

La API puede proteger los endpoints `/jobs` con token bearer:

```bash
API_TOKEN=dev-secret podman-compose up --build
```

Con token activo, las consultas deben enviar:

```bash
Authorization: Bearer dev-secret
```

Los servicios principales tienen healthchecks en Compose. El scheduler tambien
reintenta la conexion a PostgreSQL durante el arranque para tolerar que la base
tarde unos segundos en aceptar conexiones.

> Nota: este scaffold define estructura y contratos iniciales. La logica completa de fuerza bruta se implementa por etapas.
