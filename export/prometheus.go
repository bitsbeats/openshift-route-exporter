package export

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bitsbeats/openshift-route-exporter/watch"
)

const prometheusExporterName = "prometheus"

// PrometheusExporter exposes for Prometheus
type PrometheusExporter struct {
	exportDir  string
	reloadChan chan interface{}
	callbacks  []func(error)
}

// NewPrometheusExporter creates a new PrometheusExporter
func NewPrometheusExporter(exportDir string, callbacks []func(error)) *PrometheusExporter {
	c := make(chan interface{})
	p := &PrometheusExporter{exportDir, c, callbacks}
	p.callbacks = append(p.callbacks, p.reload)
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
		fname := fmt.Sprintf("%s.json", event.Route.Spec.Host)
		fpath := filepath.Join(p.exportDir, fname)

		switch event.Type {
		case watch.Added, watch.Modified:
			w, err := os.Create(fpath)
			if err != nil {
				err := fmt.Errorf("unable to create %s %s", fpath, err)
				p.trigger(err)
				continue
			}
			config := promConfig{{
				Lables:  event.Labels,
				Targets: []string{event.Route.Spec.Host},
			}}
			err = json.NewEncoder(w).Encode(config)
			if err != nil {
				err := fmt.Errorf("unable to write %s %s", fpath, err)
				p.trigger(err)
				continue
			}
			_ = w.Close()
			log.Printf("added/modified %s, labels: %+v", event.Route.Spec.Host, event.Labels)
			p.trigger(nil)
		case watch.Deleted:
			err := os.Remove(fpath)
			if err != nil {
				err := fmt.Errorf("unable to delete %s %s", fpath, err)
				p.trigger(err)
				continue
			}
			log.Printf("deleted %s, labels: %+v", event.Route.Spec.Host, event.Labels)
			p.trigger(nil)
		}
	}
}

func (p *PrometheusExporter) trigger(err error) {
	for _, f := range p.callbacks {
		f(err)
	}
}

func (p *PrometheusExporter) reload(err error) {
	if err == nil {
		p.reloadChan <- nil
	}
}

func (p *PrometheusExporter) reloadTimer() {
	var t *time.Timer
	for range p.reloadChan {
		if t != nil {
			t.Stop()
		}
		t = time.AfterFunc(50*time.Millisecond, p.reloadNow)
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
