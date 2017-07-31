package health

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// webExporter contains the metrics gathered by the instance and its path
type webExporter struct {
	reqCnt               *prometheus.CounterVec
	reqDur, reqSz, resSz prometheus.Summary

	MetricsPath string
}

// NewwebExporter generates a new set of metrics with a certain subsystem name
func newWebExporter(subsystem string) *webExporter {
	return &webExporter{
		MetricsPath: "/metrics",
	}
}

func (p *webExporter) registerMetrics(subsystem string) {

	p.reqCnt = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "requests_total",
			Help:      "How many HTTP requests processed, partitioned by status code and HTTP method.",
		},
		[]string{"code", "method", "handler"},
	)
	prometheus.MustRegister(p.reqCnt)

	p.reqDur = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Subsystem: subsystem,
			Name:      "request_duration_seconds",
			Help:      "The HTTP request latencies in seconds.",
		},
	)
	prometheus.MustRegister(p.reqDur)

	p.reqSz = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Subsystem: subsystem,
			Name:      "request_size_bytes",
			Help:      "The HTTP request sizes in bytes.",
		},
	)
	prometheus.MustRegister(p.reqSz)

	p.resSz = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Subsystem: subsystem,
			Name:      "response_size_bytes",
			Help:      "The HTTP response sizes in bytes.",
		},
	)
	prometheus.MustRegister(p.resSz)

}

func (p *webExporter) Use() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			if c.Request().URL.String() == p.MetricsPath {
				next(c)
				return
			}

			start := time.Now()

			reqSz := make(chan int)
			go computeApproximateRequestSize(c.Request(), reqSz)

			next(c)

			status := strconv.Itoa(c.Response().Status)
			elapsed := float64(time.Since(start)) / float64(time.Second)
			resSz := float64(c.Response().Size)

			p.reqDur.Observe(elapsed)
			p.reqCnt.WithLabelValues(status, c.Request().Method, c.Request().RequestURI).Inc()
			p.reqSz.Observe(float64(<-reqSz))
			p.resSz.Observe(resSz)

			return
		}
	}
}

// From https://github.com/DanielHeckrath/gin-prometheus/blob/master/gin_prometheus.go
func computeApproximateRequestSize(r *http.Request, out chan int) {
	s := 0
	if r.URL != nil {
		s = len(r.URL.String())
	}

	s += len(r.Method)
	s += len(r.Proto)
	for name, values := range r.Header {
		s += len(name)
		for _, value := range values {
			s += len(value)
		}
	}
	s += len(r.Host)

	// N.B. r.Form and r.MultipartForm are assumed to be included in r.URL.

	if r.ContentLength != -1 {
		s += int(r.ContentLength)
	}
	out <- s
}

func (e *webExporter) Register(registery prometheus.Registerer) error {

	err = registery.Register(machineCollector{collectors: collectors})
	if err != nil {
		return err
	}
	return nil
}

func (e *webExporter) Serve(listenAddress string) error {
	const metricsPath = "/metrics"

	landingPageHTML := []byte(fmt.Sprintf(`<html>
             <head><title>Web Exporter</title></head>
             <body>
             <h1>Web Exporter</h1>
             <p><a href='%s'>Metrics</a></p>
             </body>
			 </html>`, metricsPath))

	handler := promhttp.HandlerFor(prometheus.DefaultGatherer,
		promhttp.HandlerOpts{
			ErrorLog:      log,
			ErrorHandling: promhttp.ContinueOnError,
		})

	// TODO(ts): Remove deprecated and problematic InstrumentHandler usage.
	http.Handle(metricsPath, prometheus.InstrumentHandler("prometheus", handler))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(landingPageHTML)
	})

	return http.ListenAndServe(listenAddress, nil)
}
