# Ejecucion Local

## Levantar servicios

```bash
podman-compose up --build
```

## Escalar workers

```bash
podman-compose up --build --scale worker=4
```

Cada worker tambien paraleliza internamente el rango asignado. La cantidad de
goroutines por worker se configura con `WORKER_CONCURRENCY`; si no se define,
usa la cantidad de CPUs disponibles.

```bash
WORKER_CONCURRENCY=8
```

## Configuracion De Leases

El scheduler entrega cada rango con un lease temporal. Si un worker deja de
responder antes de reportar el rango, el scheduler vuelve a poner ese rango como
pendiente cuando vence `SCHEDULER_RANGE_LEASE_TTL`.

```bash
SCHEDULER_RANGE_LEASE_TTL=30s
```

Para una demo de tolerancia a fallos, se puede bajar temporalmente el TTL, crear
un job grande, detener un worker y escalar otro worker:

```bash
podman-compose up --build --scale worker=2
podman stop tp-final-sdp_worker_1
podman-compose up --scale worker=2 -d
```

El rango que habia quedado leased se reasigna despues del vencimiento del TTL.

## Metricas

Prometheus scrapea la API y el scheduler. Para revisar las metricas directas:

```bash
curl -s http://localhost:8080/metrics
curl -s http://localhost:9100/metrics
```

Metricas propias relevantes:

- `crypto_jobs_created_total`
- `crypto_jobs_found_total`
- `crypto_ranges_assigned_total`
- `crypto_ranges_completed_total`
- `crypto_ranges_expired_total`

## Crear Job De Prueba

El siguiente ejemplo busca la cadena `ab` usando SHA-256 y un espacio de busqueda
pequeno para validar el flujo completo. La API calcula el hash de `password` y
usa automaticamente su longitud:

```bash
curl -s -X POST http://localhost:8080/jobs \
  -H 'Content-Type: application/json' \
  -d '{
    "password": "ab",
    "charset": "ab",
    "chunk_size": 2
  }'
```

```bash
curl -s http://localhost:8080/jobs/<job_id>
```

## Interfaz De Terminal

Tambien se puede crear y seguir jobs desde una interfaz interactiva de terminal.
Primero dejar los servicios corriendo:

```bash
podman-compose up --build
```

En otra terminal, abrir la interfaz dentro de Compose:

```bash
make compose-terminal
```

O ejecutarla localmente contra `localhost`:

```bash
make terminal
```

La pantalla muestra los puertos publicados y refresca el estado del job hasta
que finaliza. Por defecto usa la API directa en `http://localhost:8080`; para
usar Nginx:

```bash
API_BASE_URL=http://localhost:8088 make terminal
```

## Endpoints

- API directa: `http://localhost:8080/healthz`
- API via Nginx: `http://localhost:8088/healthz`
- Jobs: `http://localhost:8080/jobs`
- Scheduler metrics: `http://localhost:9100/metrics`
- Prometheus: `http://localhost:9091`
- Grafana: `http://localhost:3000`
