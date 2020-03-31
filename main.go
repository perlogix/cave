package main

// Featurelist
// - CLI (https://github.com/mitchellh/cli)
// - config (https://github.com/spf13/viper)
// - Noise/DHT routing/consensus (github.com/perlin-network/noise)
// - bbolt kv-store (github.com/etcd-io/bbolt)
// 		- value-types: plain, secret
// 		- network updates
// - REST API (github.com/labstack/echo)
// - Prometheus endpoint (github.com/prometheus/client_golang)
// - Web UI

//VERSION is the app version
var VERSION = "v0.0.0-devel"

// CONFIG is a global
var CONFIG *Config

func main() {
	CONFIG, err := GetConfig()
	if err != nil {
		panic(err)
	}
	_ = CONFIG

}
