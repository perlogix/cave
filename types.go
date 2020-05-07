package main

import (
	"net/http"
	"net/url"
	"time"
)

// Config type defines the file configuration data
type Config struct {
	Mode    string          `yaml:"mode"`
	Cluster ClusterConfig   `yaml:"cluster"`
	KV      KVConfig        `yaml:"kv"`
	API     APIConfig       `yaml:"api"`
	UI      UIConfig        `yaml:"ui"`
	SSL     SSLConfig       `yaml:"ssl"`
	Perf    PerfConfig      `yaml:"performance"`
	Auth    AuthConfig      `yaml:"auth"`
	Plugin  PluginAppConfig `yaml:"plugin"`
}

// Cave struct wraps all the app functions
type Cave struct {
	Config     *Config
	Logger     *Log
	Cluster    *Cluster
	KV         *KV
	KVInit     bool
	API        *API
	Auth       *AuthService
	Crypto     *Crypto
	Plugins    *Plugins
	TokenStore *TokenStore
	updates    chan Message
	sync       chan Message
	tokens     chan Message
	sharedKey  *AESKey
}

//ClusterConfig type holds the cluster interface objects.
type ClusterConfig struct {
	Port          uint16 `yaml:"port"`
	DiscoveryHost string `yaml:"discovery_host"`
	Host          string `yaml:"host"`
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
	Enable      bool   `yaml:"enable"`
	Certificate string `yaml:"ssl_certificate"`
	Key         string `yaml:"ssl_key"`
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

// PluginAppConfig type
type PluginAppConfig struct {
	PluginPath    string   `yaml:"plugin_path"`
	AllowUnsigned bool     `yaml:"allow_unsigned"`
	Blacklist     []string `yaml:"blacklist"`
	SocketPrefix  string   `yaml:"socket_prefix"`
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

// PluginConfig type
type PluginConfig struct {
	Name    string                 `yaml:"name"`
	Version string                 `yaml:"version"`
	ExeName string                 `yaml:"exe_name"`
	Type    string                 `yaml:"type"`    // authenticator, api, etc.
	SubType string                 `yaml:"subtype"` // LDAP, Basic, PAM, etc.
	Env     map[string]string      `yaml:"env"`
	Config  map[string]interface{} `yaml:"config"`
}

// APIRequest type
type APIRequest struct {
	URL       *url.URL
	Body      []byte
	Headers   http.Header
	Host      string
	UserAgent string
	Cookies   []*http.Cookie
}

// Token type
type Token struct {
	IssueTime  time.Time
	ExpireTime time.Time
	UID        string
	Token      string
	IssuedBy   string
	Type       string
}
