package registery

type Registry interface {
	Register(serviceKey string, provider Provider)
	Unregister(serviceKey string, provider Provider)
	GetServiceList(serviceKey string) []Provider
	Watch(serviceKey string) chan []Provider
	Unwatch(serviceKey string, ch chan []Provider)
}

type Provider struct {
	ProviderKey string
	Network     string
	Addr        string
	Meta        map[string]string
}
