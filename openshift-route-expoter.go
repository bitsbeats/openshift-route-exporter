package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/bitsbeats/openshift-route-exporter/export"
	"github.com/bitsbeats/openshift-route-exporter/watch"
)

func main() {
	// exit handler
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// config
	exportDir := "/etc/prometheus/kube.d/"
	if exportDirFromEnv, ok := os.LookupEnv("EXPORT_DIR"); ok {
		exportDir = exportDirFromEnv
	}
	if s, err := os.Stat(exportDir); os.IsNotExist(err) || !s.IsDir() {
		log.Fatalf("unable to open %s for writing.", exportDir)
	}
	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	if kubeconfigFromEnv, ok := os.LookupEnv("KUBE_CONFIG"); ok {
		kubeconfig = kubeconfigFromEnv
	}
	exporterName := "prometheus"
	if exporterNameFromEnv, ok := os.LookupEnv("EXPORTER"); ok {
		exporterName = exporterNameFromEnv
	}
	exporter, err := export.NewExporter(exporterName, exportDir)
	if err != nil {
		log.Fatal(err)
	}

	// start watchers
	wg := sync.WaitGroup{}
	sink := make(chan watch.Event)
	kubeconfigs := strings.Split(kubeconfig, ";")
	for _, kc := range kubeconfigs {
		wg.Add(1)
		go consume(ctx, kc, sink, &wg)
	}
	go func() {
		wg.Wait()
		close(sink)
	}()

	// consume watchers
	done := make(chan interface{})
	go func() {
		exporter.Consume(sink)
		done <- nil
	}()

	// exit handler
	select {
	case <-done:
		log.Println("watcher has finished")
	case s := <-sig:
		log.Printf("received signal %s, stopping", s.String())
		cancel()
		<-done
	}
}

// consume a watcher, restart it on connection loss
func consume(ctx context.Context, kc string, sink chan watch.Event, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		log.Printf("consuming %s", kc)
		w, err := watch.NewWatcher(kc, map[string]string{"test": "text"})
		if err != nil {
			log.Fatal(err)
		}
		c, err := w.Watch(ctx)
		if err != nil {
			log.Fatal(err)
		}
		for event := range c {
			sink <- event
		}
		if w.Cancelled {
			log.Printf("consume %s cancelled", kc)
			return
		}
	}
}
