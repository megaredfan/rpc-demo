package memory

import (
	"errors"
	"github.com/google/uuid"
	"github.com/megaredfan/rpc-demo/registry"
	"sync"
	"time"
)

var (
	timeout = time.Millisecond * 10
)

type Registry struct {
	mu        sync.RWMutex
	providers map[string][]registry.Provider
	watchers  map[string]*Watcher
}

func (r *Registry) Init() {
	r.providers = make(map[string][]registry.Provider)
}

func (r *Registry) Register(serviceKey string, provider registry.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	go r.sendWatcherEvent(registry.Create, serviceKey, provider)

	list, ok := r.providers[serviceKey]
	if ok {
		list = append(list, provider)
		r.providers[serviceKey] = list
	} else {
		list = []registry.Provider{provider}
		r.providers[serviceKey] = list
	}
}

func (r *Registry) sendWatcherEvent(action registry.EventAction, serviceKey string, provider registry.Provider) {
	var watchers []*Watcher
	event := &registry.Event{
		Action:     action,
		ServiceKey: serviceKey,
		Provider:   provider,
	}
	r.mu.RLock()
	for _, w := range r.watchers {
		watchers = append(watchers, w)
	}
	r.mu.RUnlock()

	for _, w := range watchers {
		select {
		case <-w.exit:
			r.mu.Lock()
			delete(r.watchers, w.id)
			r.mu.Unlock()
		default:
			select {
			case w.res <- event:
			case <-time.After(timeout):
			}
		}
	}
}

func (r *Registry) Unregister(serviceKey string, provider registry.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	go r.sendWatcherEvent(registry.Delete, serviceKey, provider)

	list, ok := r.providers[serviceKey]
	if ok {
		var newList []registry.Provider
		for _, p := range list {
			if p.ProviderKey != provider.ProviderKey {
				newList = append(newList, p)
			}
		}
		r.providers[serviceKey] = newList
	}
}

func (r *Registry) GetServiceList(serviceKey string) []registry.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.providers[serviceKey]
}

func (r *Registry) Watch(serviceKey string) registry.Watcher {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.watchers == nil {
		r.watchers = make(map[string]*Watcher)
	}
	event := make(chan *registry.Event)
	exit := make(chan bool)
	id := uuid.New().String()

	w := &Watcher{
		id:         id,
		serviceKey: serviceKey,
		res:        event,
		exit:       exit,
	}

	r.watchers[id] = w
	return w
}

func (r *Registry) Unwatch(watcher registry.Watcher) {
	target, ok := watcher.(*Watcher)
	if !ok {
		return
	}

	r.mu.Lock()
	defer r.mu.Lock()

	var newWatcherList []registry.Watcher
	for _, w := range r.watchers {
		if w.id != target.id {
			newWatcherList = append(newWatcherList, w)
		}
	}
}

type Watcher struct {
	id         string
	serviceKey string
	res        chan *registry.Event
	exit       chan bool
}

func (m *Watcher) Next() (*registry.Event, error) {
	for {
		select {
		case r := <-m.res:
			if m.serviceKey != "" && m.serviceKey != r.ServiceKey {
				continue
			}
			return r, nil
		case <-m.exit:
			return nil, errors.New("watcher stopped")
		}
	}
}

func (m *Watcher) Close() {
	select {
	case <-m.exit:
		return
	default:
		close(m.exit)
	}
}

func NewInMemoryRegistry() registry.Registry {
	r := &Registry{}
	r.Init()
	return r
}
