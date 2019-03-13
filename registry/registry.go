package registry

type Registry interface {
	Init()
	Register(serviceKey string, provider Provider)
	Unregister(serviceKey string, provider Provider)
	GetServiceList(serviceKey string) []Provider
	Watch(serviceKey string) Watcher
	Unwatch(watcher Watcher)
}

type Watcher interface {
	Next() (*Event, error)
	Close()
}

type EventAction byte

const (
	Create EventAction = iota
	Update
	Delete
)

type Event struct {
	action     EventAction
	serviceKey string
	providers  Provider
}

type Provider struct {
	ProviderKey string // Network+"@"+Addr
	Network     string
	Addr        string
	Meta        map[string]string
}

type Peer2PeerDiscovery struct {
	providers []Provider
}

func (p *Peer2PeerDiscovery) Init() {
	p.providers = []Provider{}
}

func (p *Peer2PeerDiscovery) Register(serviceKey string, provider Provider) {
	p.providers = []Provider{provider}
}

func (p *Peer2PeerDiscovery) Unregister(serviceKey string, provider Provider) {
	p.Init()
}

func (p *Peer2PeerDiscovery) GetServiceList(serviceKey string) []Provider {
	return p.providers
}

func (p *Peer2PeerDiscovery) Watch(serviceKey string) chan []Provider {
	return nil
}

func (p *Peer2PeerDiscovery) WithProvider(provider Provider) *Peer2PeerDiscovery {
	p.providers = append(p.providers, provider)
	return p
}

func (p *Peer2PeerDiscovery) WithProviders(providers []Provider) *Peer2PeerDiscovery {
	for _, provider := range providers {
		p.providers = append(p.providers, provider)
	}
	return p
}

func NewPeer2PeerRegistry() *Peer2PeerDiscovery {
	r := &Peer2PeerDiscovery{}
	return r
}
