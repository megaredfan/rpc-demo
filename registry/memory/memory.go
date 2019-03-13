package memory

import (
	"errors"
	"github.com/google/uuid"
	"github.com/megaredfan/rpc-demo/registry"
	"sync"
	"time"
)

type Registry struct {
	mu        sync.RWMutex
	providers map[string][]registry.Provider
	watchers  map[string]*registry.Watcher
}

type Watcher interface {
	Next() (*registry.Event, error)
	Close()
}

func (r *Registry) Init() {
	panic("implement me")
}

func (r *Registry) Register(serviceKey string, provider registry.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	go sendWatcherEvent(serviceKey, provider)

	list, ok := r.providers[serviceKey]
	if ok {
		list = append(list, provider)
	} else {
		list = []registry.Provider{provider}
		r.providers[serviceKey] = list
	}
}

func (r *Registry) sendWatcherEvent(serviceKey string, provider registry.Provider) {
	var watchers []*registry.Watcher

	r.mu.RLock()
	for _, w := range r.watchers {
		watchers = append(watchers, w)
	}
	r.mu.RUnlock()

	for _, w := range watchers {
		select {
		case <-w.exit:
			m.Lock()
			delete(m.Watchers, w.id)
			m.Unlock()
		default:
			select {
			case w.res <- r:
			case <-time.After(timeout):
			}
		}
	}
}

func (r *Registry) Unregister(serviceKey string, provider registry.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
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
	defer r.mu.Lock()

	event := make(chan *registry.Event)
	exit := make(chan bool)
	id := uuid.New().String()

	w := &memoryWatcher{
		id:         id,
		serviceKey: serviceKey,
		res:        event,
		exit:       exit,
	}

	r.watchers[id] = w
	return w
}

func (r *Registry) Unwatch(watcher registry.Watcher) {
	mw := watcher.(*memoryWatcher)
	r.mu.Lock()
	defer r.mu.Lock()

	var newWatcherList []registry.Watcher
	for _, w := range r.watchers {
		if target, ok := w.(*memoryWatcher); ok {
			if target.id != mw.id {
				newWatcherList = append(newWatcherList, target)
			}
		}
	}
}

type memoryWatcher struct {
	id         string
	serviceKey string
	res        chan *registry.Event
	exit       chan bool
}

func (mw *memoryWatcher) Next() (*registry.Event, error) {
	for {
		select {
		case r := <-mw.res:
			return r, nil
		case <-mw.exit:
			return nil, errors.New("watcher stopped")
		}
	}
}

func (mw *memoryWatcher) Close() {
	select {
	case <-mw.exit:
		return
	default:
		close(mw.exit)
	}
}
