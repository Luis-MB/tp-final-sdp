# Secuencia De Job

```mermaid
sequenceDiagram
  participant C as Cliente
  participant A as API Gateway
  participant S as Scheduler
  participant W as Worker
  participant P as PostgreSQL

  C->>A: Crear job(hash, charset, longitud)
  A->>S: CreateJob(hash, charset, longitud, chunk)
  S->>S: Particionar espacio de busqueda
  S->>P: Persistir job y rangos
  W->>S: Pedir rango
  S->>S: Reservar siguiente rango
  S->>P: Persistir lease
  S-->>W: Rango asignado
  W->>W: Probar candidatos
  W->>S: Reportar rango
  S->>P: Guardar progreso
  alt Solucion encontrada
    S->>S: Marcar job como encontrado
    S->>P: Guardar resultado
  end
```
