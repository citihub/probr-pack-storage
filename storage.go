package pack

import (
	azureaw "github.com/citihub/probr-pack-storage/internal/azure/access_whitelisting"
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
			azureaw.Probe,
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
	pkger.Include("/internal/azure/access_whitelisting/access_whitelisting.feature")
	pkger.Include("/internal/azure/encryption_at_rest/encryption_at_rest.feature")
	pkger.Include("/internal/azure/encryption_in_flight/encryption_in_flight.feature")
}
