package version

var Version string

func Get() string {
	if Version != "" {
		return Version
	}
	return "dev"
}
