package main

import (
	"time"

	"gopkg.in/logex.v1"
)

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
	CONFIG, err := getConfig()
	if err != nil {
		panic(err)
	}
	log := logex.NewLogger(1)
	app := &Bunker{
		Config: CONFIG,
		Logger: log,
	}
	cluster, err := newCluster(app)
	if err != nil {
		panic(err)
	}
	app.Cluster = cluster
	go app.Cluster.Start()
	for {
		time.Sleep(2 * time.Second)
		app.Cluster.Emit(Message{
			Type:   "greeting",
			Origin: app.Cluster.node.ID().Address,
		})
	}
}
