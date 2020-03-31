package main

// Config type defines the file configuration data
type Config struct {
	Mode    string  `yaml:"mode"`
	Cluster Cluster `yaml:"cluster"`
	KV      KV      `yaml:"kv"`
	API     API     `yaml:"api"`
	UI      UI      `yaml:"ui"`
	SSL     SSL     `yaml:"ssl"`
}

// Bunker struct wraps all the app functions
type Bunker struct {
	Config *Config
	Logger *Logger
}

//Cluster type holds the cluster interface objects.
type Cluster struct {
	Port          uint16 `yaml:"port"`
	DiscoveryHost string `yaml:"discovery_host"`
	AdvertiseHost string `yaml:"advertise_host"`
}

//KV type holds the key-value engine objects.
type KV struct {
	Encryption  bool   `yaml:"enable_encryption"`
	Persist     bool   `yaml:"persist_to_disk"`
	PersistPath string `yaml:"persist_path"`
}

//API type holds the API engine objects
type API struct {
	Enable         bool   `yaml:"enable"`
	Port           uint16 `yaml:"port"`
	Authentication bool   `yaml:"authentication"`
	EnableMetrics  bool   `yaml:"enable_metrics"`
}

//UI struct holds the UI engine objects
type UI struct {
	Enable         bool   `yaml:"enable"`
	Port           uint16 `yaml:"port"`
	Authentication bool   `yaml:"authentication"`
}

//Logger handles all the logging facilities
type Logger struct {
}

//SSL holds the SSL configuration
type SSL struct {
	Enable         bool   `yaml:"enable"`
	SSLCertificate string `yaml:"ssl_certificate"`
	SSLKey         string `yaml:"ssl_key"`
}
