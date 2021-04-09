package pack

import (
	azureana "github.com/citihub/probr-pack-storage/internal/azure/allowed_network_access"
	azureear "github.com/citihub/probr-pack-storage/internal/azure/encryption_at_rest"
	azureeif "github.com/citihub/probr-pack-storage/internal/azure/encryption_in_flight"
	"github.com/citihub/probr-sdk/config"
	"github.com/citihub/probr-sdk/probeengine"
	"github.com/markbates/pkger"
)

// GetProbes returns a list of probe objects
func GetProbes() []probeengine.Probe {
	if config.Vars.ServicePacks.Storage.IsExcluded() {
		return nil
	}
	switch config.Vars.ServicePacks.Storage.Provider {
	case "Azure":
		return []probeengine.Probe{
			azureana.Probe,
			azureear.Probe,
			azureeif.Probe,
		}
	default:
		return nil
	}
}

func init() {
	// This line will ensure that all static files are bundled into pked.go file when using pkger cli tool
	// See: https://github.com/markbates/pkger
	pkger.Include("/internal/azure/allowed_network_access/allowed_network_access.feature")
	pkger.Include("/internal/azure/encryption_at_rest/encryption_at_rest.feature")
	pkger.Include("/internal/azure/encryption_in_flight/encryption_in_flight.feature")
}
