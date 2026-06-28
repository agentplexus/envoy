package config

import (
	"github.com/plexusone/omnimemory/core"
)

// DefaultRecallMax is the default maximum memories to recall per request.
const DefaultRecallMax = 5

// ToClientConfig converts MemoryConfig to omnimemory ClientConfig.
func (c *MemoryConfig) ToClientConfig() core.ClientConfig {
	if !c.Enabled {
		return core.ClientConfig{}
	}

	providerName := core.ProviderName(c.Provider)
	if !providerName.Valid() {
		providerName = core.ProviderNameMemory
	}

	return core.ClientConfig{
		Providers: []core.ProviderConfig{
			{
				Name:     providerName,
				DSN:      c.DSN,
				APIKey:   c.APIKey,
				Endpoint: c.Endpoint,
				Options:  c.Options,
			},
		},
	}
}

// GetRecallMax returns the recall max, defaulting to DefaultRecallMax if not set.
func (c *MemoryConfig) GetRecallMax() int {
	if c.RecallMax <= 0 {
		return DefaultRecallMax
	}
	return c.RecallMax
}
