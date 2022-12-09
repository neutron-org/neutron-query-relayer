package registry

// RegistryConfig represents the config structure for the Registry.
type RegistryConfig struct {
	Addresses []string
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

// Registry is the relayer's watch list registry. It contains a list of addresses, and the relayer
// only works with interchain queries that are under these addresses' ownership.
type Registry struct {
	addresses map[string]struct{}
}

// IsEmpty returns true if the registry addresses list is empty.
func (r *Registry) IsEmpty() bool {
	return len(r.addresses) == 0
}

// Contains returns true if the addr is in the registry.
func (r *Registry) Contains(addr string) bool {
	_, ex := r.addresses[addr]
	return ex
}

func (r *Registry) GetAddresses() []string {
	var out []string
	for addr := range r.addresses {
		out = append(out, addr)
	}

	return out
}
