// Package config holds tmh's profiles and where their credentials live.
package config

// Profile pairs a name with the Environment its credentials belong to.
// Credentials never work across Environments, so each has its own profile.
type Profile struct {
	Name string
	// Environment is the glossary name: Sandbox or Live.
	Environment string
	BaseURL     string
}

var (
	Sandbox = Profile{Name: "sandbox", Environment: "Sandbox", BaseURL: "https://api-teachmehipaa-dev.web.app/v1"}
	Live    = Profile{Name: "live", Environment: "Live", BaseURL: "https://api.tme.so/v1"}
)

// Default is Sandbox, so nobody reaches Live by accident.
var Default = Sandbox

func LookupProfile(name string) (Profile, bool) {
	switch name {
	case Sandbox.Name:
		return Sandbox, true
	case Live.Name:
		return Live, true
	}
	return Profile{}, false
}
