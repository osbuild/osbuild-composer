package ostreeserver

type OSTreeServer struct {
	Port       string `yaml:"port"`
	ConfigPath string `yaml:"config_path"`
}
