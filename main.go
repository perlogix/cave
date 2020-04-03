package main

import (
	"time"
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

// TERMINATOR holds signal channels for goroutines
var TERMINATOR []chan bool

func main() {
	CONFIG, err := getConfig()
	if err != nil {
		panic(err)
	}
	log := Log{}.New()
	app := &Bunker{
		Config: CONFIG,
		Logger: log,
	}
	cluster, err := newCluster(app)
	if err != nil {
		panic(err)
	}
	TERMINATOR = append(TERMINATOR, cluster.terminate)
	app.Cluster = cluster
	app.events = make(chan Message, CONFIG.Perf.BufferSize)
	app.sync = make(chan Message, CONFIG.Perf.BufferSize)
	app.updates = make(chan Message, CONFIG.Perf.BufferSize)
	err = app.Cluster.registerHandlers(app.events, app.updates, app.sync)
	if err != nil {
		panic(err)
	}
	kv, err := newKV(app)
	if err != nil {
		panic(err)
	}
	app.KV = kv
	TERMINATOR = append(TERMINATOR, kv.terminate)

	// START SHIT
	go app.Cluster.Start()
	go app.KV.Start()
	time.Sleep(60 * time.Minute)
}
