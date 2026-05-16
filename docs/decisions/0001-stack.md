# ADR 0001 - Stack Inicial

## Decision

Usar Go, gRPC, Redis, PostgreSQL, Nginx, Podman Compose, Prometheus y Grafana.

## Motivo

Go permite workers concurrentes y servicios livianos. gRPC define contratos claros entre scheduler y workers. Redis resuelve coordinacion de baja latencia. PostgreSQL aporta persistencia durable y consultable. Prometheus/Grafana permiten justificar rendimiento con metricas.

## Alternativas

- Java + Spring Boot: mas pesado para el alcance inicial.
- Python: simple para prototipar, menos conveniente para concurrencia intensiva.
- Kafka/Flink/Spark: utiles para arquitecturas mayores, pero agregan complejidad para la primera version.
