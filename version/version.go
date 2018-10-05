package version

// Version components
const (
	Maj = "0"
	Min = "22"
	Fix = "0"
)

var (
	// Version is the current version of Demars-DMC
	// Must be a string because scripts like dist.sh read this file.
	Version = "0.0.1"

	// GitCommit is the current HEAD set using ldflags.
	GitCommit string
)

func init() {
	if GitCommit != "" {
		Version += "-" + GitCommit
	}
}
