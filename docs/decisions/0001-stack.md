# ADR 0001 - Stack Inicial

## Decision

Usar Go, gRPC, PostgreSQL, Nginx, Podman Compose, Prometheus y Grafana.

## Motivo

Go permite workers concurrentes y servicios livianos. gRPC define contratos claros entre scheduler y workers. PostgreSQL aporta persistencia durable y consultable para jobs, rangos, leases y resultados. Prometheus/Grafana permiten justificar rendimiento con metricas.

## Alternativas

- Java + Spring Boot: mas pesado para el alcance inicial.
- Python: simple para prototipar, menos conveniente para concurrencia intensiva.
- Kafka/Flink/Spark: utiles para arquitecturas mayores, pero agregan complejidad para la primera version.
