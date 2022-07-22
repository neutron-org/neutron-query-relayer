package registry

// RegistryConfig represents the config structure for the Registry.
type RegistryConfig struct {
	Addresses []string `yaml:"addresses" env-required:"true"`
}

// New instantiates a new *Registry based on the cfg.
func New(cfg *RegistryConfig) *Registry {
	r := &Registry{
		addresses: make(map[string]struct{}, len(cfg.Addresses)),
	}
	for _, addr := range cfg.Addresses {
		r.addresses[addr] = struct{}{}
	}
	return r
}

// Registry is the relayer watch list registry. It contais a list of addresses, and the relayer
// relays only works with interchain queries registered by these addresses.
type Registry struct {
	addresses map[string]struct{}
}

// Contains returns true if the addr is in the registry.
func (r *Registry) Contains(addr string) bool {
	_, ex := r.addresses[addr]
	return ex
}
