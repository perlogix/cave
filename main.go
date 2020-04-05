package main

import (
	"os"
	"os/signal"
	"syscall"
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
var TERMINATOR map[string]chan bool

func main() {
	TERMINATOR = map[string]chan bool{}
	kill := make(chan os.Signal)
	signal.Notify(kill, syscall.SIGKILL, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	CONFIG, err := getConfig()
	if err != nil {
		panic(err)
	}
	log := Log{}.New(CONFIG)
	TERMINATOR["log"] = log.terminator
	go log.Start()
	app := &Bunker{
		Config: CONFIG,
		Logger: log,
	}
	cluster, err := newCluster(app)
	if err != nil {
		panic(err)
	}
	TERMINATOR["cluster"] = cluster.terminate
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
	TERMINATOR["kv"] = kv.terminate
	api, err := NewAPI(app)
	if err != nil {
		panic(err)
	}
	app.API = api
	TERMINATOR["api"] = api.terminate
	// START SHIT
	go app.Cluster.Start()
	go app.KV.Start()
	go app.API.Start()
	<-kill
	log.Warn("Got kill signal from OS, shutting down...")
	for _, t := range []string{"api", "kv", "cluster", "log"} {
		log.Warn("Shutting down " + t)
		TERMINATOR[t] <- true
	}

}
