package health

import (
	"github.com/rai-project/config"
	nvidiaexporter "github.com/rai-project/nvidia_exporter"
)

const GPUMetricName = "GPU"

func init() {
	config.AfterInit(func() {
		Config.Wait()
		if !captureMetricQ(GPUMetricName) {
			return
		}
		exporter, err := nvidiaexporter.New()
		if err != nil {
			return
		}
		_ = exporter
		// exporter.Register()
	})
}
