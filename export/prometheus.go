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
	v1 "github.com/openshift/api/route/v1"
	kwatch "k8s.io/apimachinery/pkg/watch"
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
		r, ok := event.Event.Object.(*v1.Route)
		if !ok {
			err := fmt.Errorf("unkown object %+v", event.Event.Object)
			p.trigger(err)
			continue
		}
		fname := fmt.Sprintf("%s.json", r.Spec.Host)
		fpath := filepath.Join(p.exportDir, fname)

		switch event.Event.Type {
		case kwatch.Added, kwatch.Modified:
			w, err := os.Create(fpath)
			if err != nil {
				err := fmt.Errorf("unable to create %s", fpath)
				p.trigger(err)
				continue
			}
			config := promConfig{{
				Lables:  event.Labels,
				Targets: []string{r.Spec.Host},
			}}
			err = json.NewEncoder(w).Encode(config)
			if err != nil {
				fmt.Errorf("unable to write %s", fpath)
				p.trigger(err)
				continue
			}
			_ = w.Close()
			log.Printf("added/modified %s, labels: %+v", r.Spec.Host, event.Labels)
			p.trigger(nil)
		case kwatch.Deleted:
			err := os.Remove(fpath)
			if err != nil {
				fmt.Errorf("unable to delete %s", fpath)
				p.trigger(err)
				continue
			}
			log.Printf("deleted %s, labels: %+v", r.Spec.Host, event.Labels)
			p.trigger(nil)
		case kwatch.Error:
			log.Printf("error %+v", r.Spec.Host)
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
	for _ = range p.reloadChan {
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
