package microchip

import (
	"fmt"
)

// RegistryInfo contains metadata about a pet recovery registry or clearinghouse.
type RegistryInfo struct {
	RegistryName  string `json:"registryName"`
	Clearinghouse string `json:"clearinghouse"`
	Phone         string `json:"phone,omitempty"`
	Website       string `json:"website,omitempty"`
}

const aahaClearinghouse = "AAHA Clearinghouse"

var icarPrefixMap = map[string]RegistryInfo{
	"985": {
		RegistryName:  "HomeAgain",
		Clearinghouse: aahaClearinghouse,
		Phone:         "1-888-466-3242",
		Website:       "homeagain.com",
	},
	"981": {
		RegistryName:  "AKC Reunite",
		Clearinghouse: aahaClearinghouse,
		Phone:         "1-800-252-7894",
		Website:       "akcreunite.org",
	},
	"977": {
		RegistryName:  "PetLink",
		Clearinghouse: aahaClearinghouse,
		Phone:         "1-877-738-5465",
		Website:       "petlink.net",
	},
	"982": {
		RegistryName:  "24Petwatch",
		Clearinghouse: aahaClearinghouse,
		Phone:         "1-866-597-2424",
		Website:       "24petwatch.com",
	},
	"965": {
		RegistryName:  "BuddyID",
		Clearinghouse: aahaClearinghouse,
		Phone:         "1-800-434-2843",
		Website:       "buddyid.com",
	},
}

var avidRegistry = RegistryInfo{
	RegistryName:  "Avid Identification Systems",
	Clearinghouse: aahaClearinghouse,
	Phone:         "1-800-336-2843",
	Website:       "avidid.com",
}

var euroRegistry = RegistryInfo{
	RegistryName:  "Euro / Trovan Microchip Registry",
	Clearinghouse: "Universal Clearinghouse",
	Website:       "trovan.com",
}

var defaultRegistry = RegistryInfo{
	RegistryName:  "Universal Clearinghouse",
	Clearinghouse: aahaClearinghouse,
	Website:       "aaha.org/petmicrochiplookup",
}

// IdentifyIssuingRegistry identifies the issuing pet registry or clearinghouse
// based on standard prefix routing rules and ICAR manufacturer codes.
func IdentifyIssuingRegistry(normalizedID string) RegistryInfo {
	val := ValidateAndNormalize(normalizedID)
	if !val.Valid {
		return defaultRegistry
	}

	switch val.Standard {
	case StandardAvid9:
		return avidRegistry
	case StandardEuro10:
		return euroRegistry
	default:
		prefix := val.NormalizedID[:3]
		if info, ok := icarPrefixMap[prefix]; ok {
			return info
		}
		return defaultRegistry
	}
}

// MaskMicrochip formats a microchip transponder string for zero-PII public display,
// extracting the last 4 characters and prefixing with the registry/standard label
// (e.g. "HomeAgain ••••3456", "Avid ••••6789", "Euro ••••12B3").
// If the input is empty or invalid, it returns an empty string.
func MaskMicrochip(raw string) string {
	val := ValidateAndNormalize(raw)
	if !val.Valid {
		return ""
	}

	lastFour := val.NormalizedID[len(val.NormalizedID)-4:]

	var label string
	switch val.Standard {
	case StandardAvid9:
		label = "Avid"
	case StandardEuro10:
		label = "Euro"
	default:
		reg := IdentifyIssuingRegistry(val.NormalizedID)
		if reg.RegistryName == "Universal Clearinghouse" {
			label = "ISO"
		} else {
			label = reg.RegistryName
		}
	}

	return fmt.Sprintf("%s ••••%s", label, lastFour)
}
