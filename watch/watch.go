package watch

import (
	"context"
	"log"
	"sync"
	"time"

	routev1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/clientcmd"
)

type (
	// Labels
	Labels map[string]string

	// Config to create a watcher
	Config struct {
		Kubeconfig string `yaml:"kubeconfig"`
		Labels     Labels `yaml:"labels"`
	}

	// Watcher
	Watcher struct {
		clientset  *routev1.RouteV1Client
		kubeconfig string
		Labels     Labels
		Cancelled  bool
	}

	// Multiwatcher
	MultiWatcher struct {
		watchers []*Watcher
		Sink     chan Event
	}

	// Event is a wrapper to provide the Openshift event and the labels of the Watcher
	Event struct {
		Event  watch.Event
		Labels Labels
	}
)

// NewWatcher crates a Wachter
func NewWatcher(c Config) (w *Watcher, err error) {
	config, err := clientcmd.BuildConfigFromFlags("", c.Kubeconfig)
	if err != nil {
		return
	}
	clientset, err := routev1.NewForConfig(config)
	if err != nil {
		return
	}

	return &Watcher{clientset, c.Kubeconfig, c.Labels, false}, nil
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
				c <- Event{event, w.Labels}
			case <-ctx.Done():
				break outer
			}
		}
		w.Cancelled = true
		close(c)
	}()
	return
}

// NewMultiWatcher creates a watcher for multiple configs into one chan
func NewMultiWatcher(configs []Config) (mw *MultiWatcher, err error) {
	mw = &MultiWatcher{
		[]*Watcher{},
		make(chan Event),
	}
	for _, c := range configs {
		w, err := NewWatcher(c)
		mw.watchers = append(mw.watchers, w)
		if err != nil {
			return nil, err
		}
	}
	return
}

// Watch nonblocking all Watchers and throw them into a single mw.Sink
func (mw *MultiWatcher) Watch(ctx context.Context) {
	wg := sync.WaitGroup{}
	for _, w := range mw.watchers {
		wg.Add(1)
		go func(w *Watcher) {
			defer wg.Done()
			for {
				c, err := w.Watch(ctx)
				if err != nil {
					log.Printf("unable to connect to %s \"%s\", retrying", w.kubeconfig, err)
					select {
					case <-time.After(time.Second):
						continue
					case <-ctx.Done():
						return
					}
				}
				for event := range c {
					mw.Sink <- event
				}
				if w.Cancelled {
					log.Printf("consume %s cancelled", w.kubeconfig)
					return
				}
			}
		}(w)
	}
	go func() {
		wg.Wait()
		close(mw.Sink)
	}()
}
