# Arquitectura

```mermaid
flowchart LR
  Client[Cliente] --> Nginx[Nginx]
  Nginx --> API[API Gateway]
  API --> Scheduler[Scheduler gRPC]
  Scheduler --> Postgres[(PostgreSQL)]
  Worker1[Worker] --> Scheduler
  Worker2[Worker] --> Scheduler
  WorkerN[Worker N] --> Scheduler
  API --> Prometheus[Prometheus]
  Prometheus --> Grafana[Grafana]
```

## Responsabilidades

- API Gateway: expone endpoints HTTP para usuarios y herramientas de prueba.
- Scheduler: asigna rangos, recibe reportes y emite cancelacion global.
- Worker: procesa rangos de candidatos y reporta resultados.
- PostgreSQL: conserva jobs, rangos, leases y resultados para auditoria,
  recuperacion y analisis.
