package health

import "strings"

func captureMetricQ(metricName string) bool {
	metricName = strings.ToLower(metricName)
	for _, toCapture := range Config.Metrics {
		toCapture = strings.ToLower(toCapture)
		if metricName == toCapture {
			return true
		}
	}
	return false
}
