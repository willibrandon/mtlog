package enrichers

import (
	"os"
	
	"github.com/willibrandon/mtlog/core"
)

// EnvironmentEnricher adds environment variable values to log events.
type EnvironmentEnricher struct {
	variableName string
	propertyName string
	cached       bool
	cachedValue  string
}

// NewEnvironmentEnricher creates an enricher that adds the value of an environment variable.
func NewEnvironmentEnricher(variableName, propertyName string) *EnvironmentEnricher {
	return &EnvironmentEnricher{
		variableName: variableName,
		propertyName: propertyName,
	}
}

// NewEnvironmentEnricherCached creates an enricher that caches the environment variable value.
func NewEnvironmentEnricherCached(variableName, propertyName string) *EnvironmentEnricher {
	enricher := &EnvironmentEnricher{
		variableName: variableName,
		propertyName: propertyName,
		cached:       true,
	}
	enricher.cachedValue = os.Getenv(variableName)
	return enricher
}

// Enrich adds the environment variable value to the log event.
func (e *EnvironmentEnricher) Enrich(event *core.LogEvent, propertyFactory core.LogEventPropertyFactory) {
	var value string
	if e.cached {
		value = e.cachedValue
	} else {
		value = os.Getenv(e.variableName)
	}
	
	if value != "" {
		prop := propertyFactory.CreateProperty(e.propertyName, value)
		event.Properties[prop.Name] = prop.Value
	}
}

// CommonEnvironmentEnrichers returns enrichers for common environment variables.
func CommonEnvironmentEnrichers() []core.LogEventEnricher {
	return []core.LogEventEnricher{
		NewEnvironmentEnricherCached("ENVIRONMENT", "Environment"),
		NewEnvironmentEnricherCached("SERVICE_NAME", "ServiceName"),
		NewEnvironmentEnricherCached("SERVICE_VERSION", "ServiceVersion"),
		NewEnvironmentEnricherCached("DEPLOYMENT_ID", "DeploymentId"),
		NewEnvironmentEnricherCached("REGION", "Region"),
	}
}