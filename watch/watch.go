package watch

import (
	"context"

	routev1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/clientcmd"
)

type (
	// Labels
	Labels map[string]string

	// Watcher
	Watcher struct {
		clientset *routev1.RouteV1Client
		Labels    Labels
		Cancelled bool
	}

	// Event is a wrapper to provide the Openshift event and the labels of the Watcher
	Event struct {
		Event  watch.Event
		labels *Labels
	}
)

// NewWatcher crates a Wachter
func NewWatcher(kubeconfig string, labels map[string]string) (w *Watcher, err error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return
	}
	clientset, err := routev1.NewForConfig(config)
	if err != nil {
		return
	}

	return &Watcher{clientset, labels, false}, nil
}

// Watch nonblocking all events from openshift and throw them into c
func (w *Watcher) Watch(ctx context.Context) (c chan Event, err error) {
	c = make(chan Event)
	watcher, err := w.clientset.Routes("").Watch(metav1.ListOptions{})
	if err != nil {
		return
	}

	// consume channel or wait for context cancel
	go func() {
	outer:
		for {
			select {
			case event := <-watcher.ResultChan():
				c <- Event{event, &w.Labels}
			case <-ctx.Done():
				break outer
			}
		}
		w.Cancelled = true
		close(c)
	}()
	return
}

// Lables returns the labels of the Watcher that created this event.
func (e *Event) Labels() Labels {
	return *e.labels
}
