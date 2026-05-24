# Work Queue

La coordinacion actual de rangos vive en el scheduler y se persiste en
PostgreSQL. Cada rango tiene estado `pending`, `leased` o `completed`, con lease
temporal para permitir reasignacion si un worker cae.

Este paquete queda reservado para una futura extraccion de la cola de trabajo si
se necesita escalar el scheduler horizontalmente.
