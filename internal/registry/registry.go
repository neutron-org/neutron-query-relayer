package registry

// RegistryConfig represents the config structure for the Registry.
type RegistryConfig struct {
	Addresses []string
	QueryIDs  []uint64 `envconfig:"QUERY_IDS"`
}

// New instantiates a new *Registry based on the cfg.
func New(cfg *RegistryConfig) *Registry {
	r := &Registry{
		addresses: make(map[string]struct{}, len(cfg.Addresses)),
		queryIDs:  make(map[uint64]struct{}, len(cfg.QueryIDs)),
	}
	for _, addr := range cfg.Addresses {
		r.addresses[addr] = struct{}{}
	}
	for _, queryID := range cfg.QueryIDs {
		r.queryIDs[queryID] = struct{}{}
	}
	return r
}

// Registry is the relayer's watch list registry. It contains a list of addresses and a list of queryIDs,
// and the relayer only works with interchain queries that are under these addresses' ownership and match the queryIDs.
type Registry struct {
	addresses map[string]struct{}
	queryIDs  map[uint64]struct{}
}

// IsAddressesEmpty returns true if the registry addresses list is empty.
func (r *Registry) IsAddressesEmpty() bool {
	return len(r.addresses) == 0
}

// IsQueryIDsEmpty returns true if the registry queryIDs list is empty.
func (r *Registry) IsQueryIDsEmpty() bool {
	return len(r.queryIDs) == 0
}

// ContainsAddress returns true if the addr is in the registry.
func (r *Registry) ContainsAddress(addr string) bool {
	_, ex := r.addresses[addr]
	return ex
}

// ContainsQueryID returns true if the queryID is in the registry.
func (r *Registry) ContainsQueryID(queryID uint64) bool {
	_, ex := r.queryIDs[queryID]
	return ex
}

func (r *Registry) GetAddresses() []string {
	var out []string
	for addr := range r.addresses {
		out = append(out, addr)
	}

	return out
}
