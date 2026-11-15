// Package config holds tmh's profiles and where their credentials live.
package config

// Profile pairs a name with the Environment its credentials belong to.
// Credentials never work across Environments, so each has its own profile.
type Profile struct {
	Name string
	// Environment is the glossary name: Sandbox or Live.
	Environment string
	BaseURL     string
	// AuthorizeURL is the console's consent page for Partner app logins,
	// or "" where none is published.
	AuthorizeURL string
}

// The two profiles, one per Environment.
var (
	// Sandbox is for testing; it is the default.
	Sandbox = Profile{Name: "sandbox", Environment: "Sandbox", BaseURL: "https://api-teachmehipaa-dev.web.app/v1"}
	// Live is production.
	Live = Profile{
		Name: "live", Environment: "Live", BaseURL: "https://api.tme.so/v1",
		AuthorizeURL: "https://console.teachmehipaa.com/oauth/authorize",
	}
)

// Default is Sandbox, so nobody reaches Live by accident.
var Default = Sandbox

// LookupProfile finds a profile by its --profile name.
func LookupProfile(name string) (Profile, bool) {
	switch name {
	case Sandbox.Name:
		return Sandbox, true
	case Live.Name:
		return Live, true
	}
	return Profile{}, false
}
