# Roadmap

## Paso 1 - Estructura

- Scaffold de servicios.
- Contrato gRPC inicial.
- Podman Compose base.
- Documentacion de arquitectura.

## Paso 2 - Dominio Y Particionado

- Modelo de job.
- Generador de rangos.
- Conversion indice numerico -> candidato.
- Hash SHA-256 educativo.

## Paso 3 - Scheduler

- Creacion de jobs.
- Encolado de rangos en Redis.
- Persistencia inicial en PostgreSQL.
- API de estado.

## Paso 4 - Worker

- Registro con scheduler.
- Solicitud de rangos.
- Busqueda local.
- Reporte de progreso y solucion.

## Paso 5 - Tolerancia A Fallos

- Heartbeats.
- Reasignacion de rangos vencidos.
- Cancelacion global por job.
- Recuperacion desde PostgreSQL.

## Paso 6 - Metricas Y Pruebas

- Metricas Prometheus.
- Dashboard Grafana.
- Tests unitarios.
- Tests de integracion.
- Benchmarks con distinta cantidad de workers.
