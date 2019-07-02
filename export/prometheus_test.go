package export

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/bitsbeats/openshift-route-exporter/watch"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
)

func TestPrometheusExporter(t *testing.T) {
	// read testdata
	labels := map[string]string{"key": "value"}
	obj := routev1.Route{}
	r, err := os.Open("testdata/route_pometheus.json")
	if err != nil {
		t.Fatal("undable to find testdata")
	}
	err = json.NewDecoder(r).Decode(&obj)
	if err != nil {
		t.Fatalf("undable to load testdata %s", err)
	}

	// create a callback go get notified
	events := make(chan watch.Event)
	callback := make(chan error)
	e := NewPrometheusExporter("./testdata", []func(error){
		func(error) { callback <- err },
	})

	// run consumer
	go e.Consume(events)

	// check create
	f := "testdata/prometheus-prometheus.apps.example.com.yaml"
	events <- watch.Event{Route: &obj, Type: watch.Added, Labels: labels}
	err = <-callback
	if err != nil {
		t.Fatal(err)
	}
	got, err := ioutil.ReadFile(f)
	if err != nil {
		t.Fatalf("undable to find %s", f)
	}
	want := "[{\"lables\":{\"key\":\"value\"},\"targets\":[\"prometheus-prometheus.apps.example.com\"]}]\n"
	assert.Equal(t, want, string(got))

	// check modify
	labels = map[string]string{"key": "value2"}
	events <- watch.Event{Route: &obj, Type: watch.Modified, Labels: labels}
	err = <-callback
	if err != nil {
		t.Fatal(err)
	}
	got, err = ioutil.ReadFile(f)
	if err != nil {
		t.Fatalf("undable to find %s", f)
	}
	want = "[{\"lables\":{\"key\":\"value2\"},\"targets\":[\"prometheus-prometheus.apps.example.com\"]}]\n"
	assert.Equal(t, want, string(got))

	// check delete
	events <- watch.Event{Route: &obj, Type: watch.Deleted, Labels: labels}
	err = <-callback
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Open(f)
	assert.Equal(t, true, os.IsNotExist(err))

}
