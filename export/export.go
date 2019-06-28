package export

import (
	"fmt"

	"github.com/bitsbeats/openshift-route-exporter/watch"
)

type (
	// An Exporter conumes a watch.Event channel
	Exporter interface {
		// Consume parses the incoming events on the c channel.
		Consume(c <-chan watch.Event)
	}
)

func NewExporter(exporterName string, exportDir string) (e Exporter, err error) {
	switch exporterName {
	case prometheusExporterName:
		return NewPrometheusExporter(exportDir), nil
	default:
		return nil, fmt.Errorf("exporter '%s' does not exist", exporterName)
	}
}
