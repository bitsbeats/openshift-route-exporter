package export

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"strconv"
	"syscall"
	"time"

	v1 "github.com/openshift/api/route/v1"
	"github.com/bitsbeats/openshift-route-exporter/watch"
	kwatch "k8s.io/apimachinery/pkg/watch"
)

const prometheusExporterName = "prometheus"

// PrometheusExporter exposes for Prometheus
type PrometheusExporter struct {
	exportDir string
	reloadChan chan interface{}
}

// NewPrometheusExporter creates a new PrometheusExporter
func NewPrometheusExporter(exportDir string) *PrometheusExporter {
	c := make(chan interface{})
	p := &PrometheusExporter{exportDir, c}
	go p.reloadTimer()
	return p
}

type (
	promConfigItem struct {
		Lables  map[string]string `json:"lables"`
		Targets []string          `json:"targets"`
	}
	promConfig []promConfigItem
)

// Consume handles the events from c
func (p *PrometheusExporter) Consume(c <-chan watch.Event) {
	for event := range c {
		r := event.Event.Object.(*v1.Route)
		fname := fmt.Sprintf("%s.yaml", r.Spec.Host)
		fpath := filepath.Join(p.exportDir, fname)

		switch event.Event.Type {
		case kwatch.Added, kwatch.Modified:
			w, err := os.Create(fpath)
			if err != nil {
				log.Printf("unable to create %s", fpath)
				continue
			}
			config := promConfig{{
				Lables:  event.Labels(),
				Targets: []string{r.Spec.Host},
			}}
			err = json.NewEncoder(w).Encode(config)
			if err != nil {
				log.Printf("unable to write %s", fpath)
				continue
			}
			_ = w.Close()
			p.reload()
			log.Printf("added/modified %s, labels: %+v", r.Spec.Host, event.Labels())
		case kwatch.Deleted:
			err := os.Remove(fpath)
			if err != nil {
				log.Printf("unable to delete %s", fpath)
				continue
			}
			p.reload()
			log.Printf("deleted %s, labels: %+v", r.Spec.Host, event.Labels())
		case kwatch.Error:
			log.Printf("error %+v", r.Spec.Host)
		}
	}
}

func (p *PrometheusExporter) reload() {
	p.reloadChan<-nil
}

func (p *PrometheusExporter) reloadTimer() {
	var t *time.Timer
	for _ = range p.reloadChan {
		if t != nil {
			t.Stop()
		}
		t = time.AfterFunc(50 * time.Millisecond, p.reloadNow)
	}
}

func (p *PrometheusExporter) reloadNow() {
	procs, _ := filepath.Glob("/proc/*/exe")
	for _, proc := range procs {
		cmd, _ := os.Readlink(proc)
		if strings.Contains(cmd, "prometheus") {
			procsplit := strings.Split(proc, "/")
			pid, err := strconv.Atoi(procsplit[2])
			if err != nil {
				continue
			}
			p, err := os.FindProcess(pid)
			if err != nil {
				continue
			}
			err = p.Signal(syscall.SIGHUP)
			if err != nil {
				continue
			}
			log.Printf("reloaded prometheus with pid %d", pid)
		}
	}
}
