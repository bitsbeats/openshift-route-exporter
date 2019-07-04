package watch

import (
	"context"
	"log"
	"regexp"
	"sync"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	csroutev1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

type EventType int

const (
	Added EventType = iota + 1
	Modified
	Deleted
)

type (
	// Labels
	Labels map[string]string

	// Config to create a watcher
	Config struct {
		Kubeconfig          string `yaml:"kubeconfig"`
		NamespaceBlackRegex string `yaml:"namespace_blacklist_regex"`
		Labels              Labels `yaml:"labels"`
	}

	// Watcher
	Watcher struct {
		clientset           *csroutev1.RouteV1Client
		kubeconfig          string
		Labels              Labels
		Cancelled           bool
		NamespaceBlackRegex *regexp.Regexp
	}

	// Multiwatcher
	MultiWatcher struct {
		watchers []*Watcher
		Sink     chan Event
	}

	// Event is a wrapper to provide the Openshift event and the labels of the Watcher
	Event struct {
		Route  *routev1.Route
		Type   EventType
		Labels Labels
	}
)

// NewWatcher crates a Wachter
func NewWatcher(c Config) (w *Watcher, err error) {
	config, err := clientcmd.BuildConfigFromFlags("", c.Kubeconfig)
	if err != nil {
		return
	}
	clientset, err := csroutev1.NewForConfig(config)
	if err != nil {
		return
	}
	if c.NamespaceBlackRegex == "" {
		c.NamespaceBlackRegex = "^$"
	}
	re, err := regexp.Compile(c.NamespaceBlackRegex)
	if err != nil {
		return
	}

	return &Watcher{clientset, c.Kubeconfig, c.Labels, false, re}, nil
}

// Watch nonblocking all events from openshift and throw them into c
func (w *Watcher) Watch(ctx context.Context) (c chan Event, err error) {
	c = make(chan Event)
	_, controller := cache.NewInformer(
		cache.NewListWatchFromClient(
			w.clientset.RESTClient(), "routes", corev1.NamespaceAll, fields.Everything(),
		),
		&routev1.Route{},
		0*time.Second,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				r, ok := obj.(*routev1.Route)
				if !ok || !w.validRoute(r) {
					return
				}
				labels := labelConcat(r, w.Labels)
				c <- Event{r, Added, labels}
			},
			UpdateFunc: func(old, new interface{}) {
				r, ok := new.(*routev1.Route)
				if !ok || !w.validRoute(r) {
					return
				}
				c <- Event{r, Modified, w.Labels}
			},
			DeleteFunc: func(obj interface{}) {
				r, ok := obj.(*routev1.Route)
				if !ok || !w.validRoute(r) {
					return
				}
				c <- Event{r, Deleted, w.Labels}
			},
		},
	)

	// consume or wait for context cancel
	go func() {
		controller.Run(ctx.Done())
		w.Cancelled = true
		close(c)
	}()
	return c, err
}

func (w *Watcher) validRoute(r *routev1.Route) bool {
	valid := !w.NamespaceBlackRegex.MatchString(r.ObjectMeta.Namespace)
	return valid
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

func labelConcat(r *routev1.Route, l Labels) (concat Labels) {
	labels := []Labels{
		r.ObjectMeta.Labels,
		l,
		{"Namespace": r.ObjectMeta.Namespace},
	}
	concat = Labels{}
	for _, l := range labels {
		for k, v := range l {
			concat[k] = v
		}
	}
	return
}
