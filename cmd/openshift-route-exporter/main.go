package main

import (
	"context"
	"log"
	"os"
	"os/signal"
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
	configFile := "/etc/openshift-route-exporter/config.yml"
	if configFileDirFromEnv, ok := os.LookupEnv("CONFIG"); ok {
		configFile = configFileDirFromEnv
	}
	config, err := loadConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}
	if s, err := os.Stat(config.ExportDir); os.IsNotExist(err) || !s.IsDir() {
		log.Fatalf("unable to open %s for writing.", config.ExportDir)
	}

	// create exporter
	callbacks := []func(error){errhandler}
	exporter, err := export.NewExporter(config.Exporter, config.ExportDir, callbacks)
	if err != nil {
		log.Fatal(err)
	}

	// start watchers
	mw, err := watch.NewMultiWatcher(config.Targets)
	if err != nil {
		log.Fatal(err)
	}
	mw.Watch(ctx)

	// consume watchers
	done := make(chan interface{})
	go func() {
		exporter.Consume(mw.Sink)
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

func errhandler(err error) {
	if err != nil {
		log.Println(err)
	}
}
