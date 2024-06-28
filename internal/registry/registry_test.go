package registry_test

import (
	"testing"

	"github.com/neutron-org/neutron-query-relayer/internal/registry"
	"github.com/stretchr/testify/assert"
)

func TestRegistryWithEmptyAddressesAndEmptyQueryIDs(t *testing.T) {
	cfg := registry.RegistryConfig{}
	r := registry.New(&cfg)
	assert.True(t, r.IsAddressesEmpty())
	assert.True(t, r.IsQueryIDsEmpty())

	assert.False(t, r.ContainsAddress("not_exist_address"))
	assert.False(t, r.ContainsQueryID(0))
	assert.False(t, r.ContainsQueryID(1))
	assert.Equal(t, []string(nil), r.GetAddresses())
}

func TestRegistryWithAddressesAndEmptyQueryIDs(t *testing.T) {
	cfg := registry.RegistryConfig{
		Addresses: []string{"exist_address", "exist_address2"},
	}
	r := registry.New(&cfg)
	assert.False(t, r.IsAddressesEmpty())
	assert.True(t, r.ContainsAddress("exist_address"))
	assert.False(t, r.ContainsAddress("not_exist_address"))
	assert.ElementsMatch(t, []string{"exist_address", "exist_address2"}, r.GetAddresses())
}

func TestRegistryWithAddressesAndQueryIDs(t *testing.T) {
	cfg := registry.RegistryConfig{
		Addresses: []string{"exist_address", "exist_address2"},
		QueryIDs:  []uint64{0, 1},
	}
	r := registry.New(&cfg)
	assert.False(t, r.IsAddressesEmpty())
	assert.False(t, r.IsQueryIDsEmpty())
	assert.True(t, r.ContainsAddress("exist_address"))
	assert.False(t, r.ContainsAddress("not_exist_address"))
	assert.True(t, r.ContainsQueryID(0))
	assert.True(t, r.ContainsQueryID(1))
	assert.False(t, r.ContainsQueryID(2))
}

func TestRegistryWithEmptyAddressesAndQueryIDs(t *testing.T) {
	cfg := registry.RegistryConfig{
		QueryIDs: []uint64{0, 1},
	}
	r := registry.New(&cfg)
	assert.True(t, r.IsAddressesEmpty())
	assert.False(t, r.IsQueryIDsEmpty())
	assert.False(t, r.ContainsAddress("not_exist_address"))
	assert.True(t, r.ContainsQueryID(0))
	assert.True(t, r.ContainsQueryID(1))
	assert.False(t, r.ContainsQueryID(2))
}
