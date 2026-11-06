package app

import (
	"encoding/json"
	"errors"
	"regexp"
)

type DelegateSettings struct {
	Scope                    string
	Operations, Destinations []string
	Lifetime                 string
}

var destinationKey = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

// ParseDelegateConfig consumes only build/startup settings. Legacy declarations
// retain their TTL/job branch; absent extension fields never imply permanence.
func ParseDelegateConfig(cfg Config) (DelegateSettings, error) {
	out := DelegateSettings{Scope: cfg.DelegateScope, Lifetime: cfg.DelegateLifetime}
	invalid := errors.New("invalid delegate build configuration")
	if cfg.DelegateOperationsJSON != "" && json.Unmarshal([]byte(cfg.DelegateOperationsJSON), &out.Operations) != nil {
		return DelegateSettings{}, invalid
	}
	if cfg.DelegateDestinationsJSON != "" && json.Unmarshal([]byte(cfg.DelegateDestinationsJSON), &out.Destinations) != nil {
		return DelegateSettings{}, invalid
	}
	if out.Lifetime == "" {
		if len(out.Destinations) != 0 {
			return DelegateSettings{}, invalid
		}
		seen := map[string]bool{}
		for _, op := range out.Operations {
			switch op {
			case "files.list", "files.info", "files.createFolder", "files.checkPermission", "files.write", "files.read", "files.rename", "files.copy", "files.move", "files.delete":
			default:
				return DelegateSettings{}, invalid
			}
			if seen[op] {
				return DelegateSettings{}, invalid
			}
			seen[op] = true
		}
		return out, nil
	}
	if out.Lifetime != "until-revoked" || out.Scope == "" || len(out.Operations) != 1 || out.Operations[0] != "files.moveFromStaging" || len(out.Destinations) == 0 || len(out.Destinations) > 64 {
		return DelegateSettings{}, invalid
	}
	seen := map[string]bool{}
	for _, key := range out.Destinations {
		if !destinationKey.MatchString(key) || seen[key] {
			return DelegateSettings{}, invalid
		}
		seen[key] = true
	}
	return out, nil
}
