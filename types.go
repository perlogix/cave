package main

// Config type defines the file configuration data
type Config struct {
	Mode    string        `yaml:"mode"`
	Cluster ClusterConfig `yaml:"cluster"`
	KV      KVConfig      `yaml:"kv"`
	API     APIConfig     `yaml:"api"`
	UI      UIConfig      `yaml:"ui"`
	SSL     SSLConfig     `yaml:"ssl"`
	Perf    PerfConfig    `yaml:"performance"`
}

// Bunker struct wraps all the app functions
type Bunker struct {
	Config  *Config
	Logger  *LoggerConfig
	Cluster *Cluster
}

//ClusterConfig type holds the cluster interface objects.
type ClusterConfig struct {
	BindPort      uint16 `yaml:"bind_port"`
	DiscoveryHost string `yaml:"discovery_host"`
	AdvertiseHost string `yaml:"advertise_host"`
}

//KVConfig type holds the key-value engine objects.
type KVConfig struct {
	Encryption  bool   `yaml:"enable_encryption"`
	Persist     bool   `yaml:"persist_to_disk"`
	PersistPath string `yaml:"persist_path"`
}

//APIConfig type holds the API engine objects
type APIConfig struct {
	Enable         bool   `yaml:"enable"`
	Port           uint16 `yaml:"port"`
	Authentication bool   `yaml:"authentication"`
	EnableMetrics  bool   `yaml:"enable_metrics"`
}

//UIConfig struct holds the UI engine objects
type UIConfig struct {
	Enable         bool   `yaml:"enable"`
	Port           uint16 `yaml:"port"`
	Authentication bool   `yaml:"authentication"`
}

//LoggerConfig handles all the logging facilities
type LoggerConfig struct {
}

//SSLConfig holds the SSL configuration
type SSLConfig struct {
	Enable         bool   `yaml:"enable"`
	SSLCertificate string `yaml:"ssl_certificate"`
	SSLKey         string `yaml:"ssl_key"`
}

//PerfConfig holds performance configs
type PerfConfig struct {
	BufferSize uint64 `yaml:"buffer_size"`
}
