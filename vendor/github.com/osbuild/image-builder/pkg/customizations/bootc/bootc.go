package bootc

type Config struct {
	// Name of the config file
	Filename string

	// Filesystem type for the root partition
	RootFilesystemType string

	// Extra kernel args to append
	KernelArgs []string
}
