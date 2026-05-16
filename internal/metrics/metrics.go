package metrics

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var registerOnce sync.Once

var JobsCreated = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "crypto_jobs_created_total",
	Help: "Total de jobs creados.",
})

var RangesAssigned = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "crypto_ranges_assigned_total",
	Help: "Total de rangos asignados a workers.",
})

var RangesCompleted = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "crypto_ranges_completed_total",
	Help: "Total de rangos reportados como completados.",
})

var RangesExpired = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "crypto_ranges_expired_total",
	Help: "Total de leases vencidos y devueltos a pending.",
})

var JobsFound = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "crypto_jobs_found_total",
	Help: "Total de jobs resueltos con plaintext encontrado.",
})

func Register() {
	registerOnce.Do(func() {
		prometheus.MustRegister(JobsCreated)
		prometheus.MustRegister(RangesAssigned)
		prometheus.MustRegister(RangesCompleted)
		prometheus.MustRegister(RangesExpired)
		prometheus.MustRegister(JobsFound)
	})
}

func Handler() http.Handler {
	return promhttp.Handler()
}
