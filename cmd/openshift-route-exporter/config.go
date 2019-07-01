package main

import (
	"os"
	"log"

	"github.com/bitsbeats/openshift-route-exporter/watch"
	"gopkg.in/yaml.v2"
)

type config struct {
	Targets  []watch.Config `yaml:"targets"`
	Exporter string         `yaml:"exporter"`
	ExportDir string `yaml:"export_dir"`
}

func loadConfig(f string) (c *config, err error) {
	r, err := os.Open(f)
	if err != nil {
		return
	}
	c = &config{}
	yaml.NewDecoder(r).Decode(c)
	if err != nil {
		log.Fatalf("unable to parse config %s")
		return
	}
	return c, nil
}

