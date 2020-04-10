package main

import (
	"time"
)

// Config type defines the file configuration data
type Config struct {
	Mode    string        `yaml:"mode"`
	Cluster ClusterConfig `yaml:"cluster"`
	KV      KVConfig      `yaml:"kv"`
	API     APIConfig     `yaml:"api"`
	UI      UIConfig      `yaml:"ui"`
	SSL     SSLConfig     `yaml:"ssl"`
	Perf    PerfConfig    `yaml:"performance"`
	Auth    AuthConfig    `yaml:"auth"`
}

// Bunker struct wraps all the app functions
type Bunker struct {
	Config    *Config
	Logger    *Log
	Cluster   *Cluster
	KV        *KV
	KVInit    bool
	API       *API
	Auth      *AuthService
	Crypto    *Crypto
	updates   chan Message
	sync      chan Message
	sharedKey *AESKey
}

//ClusterConfig type holds the cluster interface objects.
type ClusterConfig struct {
	BindPort      uint16 `yaml:"bind_port"`
	DiscoveryHost string `yaml:"discovery_host"`
	AdvertiseHost string `yaml:"advertise_host"`
	SyncPort      uint16 `yaml:"sync_port"`
}

//KVConfig type holds the key-value engine objects.
type KVConfig struct {
	Encryption bool   `yaml:"enable_encryption"`
	DBPath     string `yaml:"db_path"`
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
	SSLPort        uint16 `yaml:"ssl_port"`
	SSLCertificate string `yaml:"ssl_certificate"`
	SSLKey         string `yaml:"ssl_key"`
}

//PerfConfig holds performance configs
type PerfConfig struct {
	EnableMetrics  bool   `yaml:"enable_metrics"`
	EnableHTTPLogs bool   `yaml:"enable_http_logs"`
	BufferSize     uint64 `yaml:"buffer_size"`
}

// AuthConfig type
type AuthConfig struct {
	Provider string `yaml:"provider"`
}

// Message type represents a message on the wire
type Message struct {
	Epoch    uint64 `json:"epoch"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Origin   string `json:"origin"`
	Data     []byte `json:"data"`
	DataType string `json:"data_type"`
}

type node struct {
	ID       string
	Address  string
	Distance time.Duration
}
