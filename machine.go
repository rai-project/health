package health

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rai-project/node_exporter/collector"
)

const (
	defaultCollectors = "arp,bcache,conntrack,cpu,diskstats,entropy,edac,exec," +
		"filefd,filesystem,hwmon,infiniband,loadavg,mdadm,meminfo,netdev,netstat," +
		"sockstat,stat,textfile,time,uname,vmstat,wifi,xfs,zfs"
)

var (
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(collector.Namespace, "scrape", "collector_duration_seconds"),
		"machine_exporter: Duration of a collector scrape.",
		[]string{"collector"},
		nil,
	)
	scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(collector.Namespace, "scrape", "collector_success"),
		"machine_exporter: Whether a collector succeeded.",
		[]string{"collector"},
		nil,
	)
)

// machineCollector implements the prometheus.Collector interface.
type machineCollector struct {
	collectors map[string]collector.Collector
}

// Describe implements the prometheus.Collector interface.
func (n machineCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

// Collect implements the prometheus.Collector interface.
func (n machineCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	wg.Add(len(n.collectors))
	for name, c := range n.collectors {
		go func(name string, c collector.Collector) {
			execute(name, c, ch)
			wg.Done()
		}(name, c)
	}
	wg.Wait()
}

func filterAvailableCollectors(collectors string) string {
	var availableCollectors []string
	for _, c := range strings.Split(collectors, ",") {
		_, ok := collector.Factories[c]
		if ok {
			availableCollectors = append(availableCollectors, c)
		}
	}
	return strings.Join(availableCollectors, ",")
}

func execute(name string, c collector.Collector, ch chan<- prometheus.Metric) {
	begin := time.Now()
	err := c.Update(ch)
	duration := time.Since(begin)
	var success float64

	if err != nil {
		log.Errorf("ERROR: %s collector failed after %fs: %s", name, duration.Seconds(), err)
		success = 0
	} else {
		log.Debugf("OK: %s collector succeeded after %fs.", name, duration.Seconds())
		success = 1
	}
	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds(), name)
	ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, success, name)
}

func loadCollectors(list string) (map[string]collector.Collector, error) {
	collectors := map[string]collector.Collector{}
	for _, name := range strings.Split(list, ",") {
		fn, ok := collector.Factories[name]
		if !ok {
			return nil, fmt.Errorf("collector '%s' not available", name)
		}
		c, err := fn()
		if err != nil {
			return nil, err
		}
		collectors[name] = c
	}
	return collectors, nil
}

func (e *machineCollector) Register(registery prometheus.Registerer) error {
	collectors, err := loadCollectors(filterAvailableCollectors(defaultCollectors))
	if err != nil {
		return err
	}

	err = registery.Register(machineCollector{collectors: collectors})
	if err != nil {
		return err
	}
	return nil
}

func (e *machineCollector) Serve(listenAddress string) error {
	const metricsPath = "/metrics"

	landingPageHTML := []byte(fmt.Sprintf(`<html>
             <head><title>Machine Exporter</title></head>
             <body>
             <h1>Machine Exporter</h1>
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
