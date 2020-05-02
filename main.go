package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"github.com/pkg/profile"
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

// TRUSTTOKEN is a randomly-generated token that allows plugins
// to communicate with the server without authenticating
var TRUSTTOKEN string

func main() {
	TRUSTTOKEN = uuid.New().String()
	var p interface{ Stop() }
	if os.Getenv("PROFILE") != "" {
		p = profile.Start(profile.ProfilePath("diag/"), profile.MemProfile)
		defer p.Stop()
	}
	TERMINATOR = map[string]chan bool{}
	kill := make(chan os.Signal)
	signal.Notify(kill, syscall.SIGKILL, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	go mainMetrics()
	CONFIG, err := getConfig()
	if err != nil {
		panic(err)
	}
	log := Log{}.New(CONFIG)
	TERMINATOR["log"] = log.terminator
	go log.Start()
	log.Debug("START: Logger")
	app := &Cave{
		Config: CONFIG,
		Logger: log,
	}
	crypto, err := newCrypto()
	if err != nil {
		panic(err)
	}
	app.Crypto = crypto
	cluster, err := newCluster(app)
	if err != nil {
		panic(err)
	}
	TERMINATOR["cluster"] = cluster.terminate
	app.Cluster = cluster
	app.updates = make(chan Message, 4096)
	app.sync = make(chan Message, 4096)
	clusterReady := make(chan bool)
	err = app.Cluster.registerHandlers(app.updates, app.sync)
	if err != nil {
		panic(err)
	}

	go app.Cluster.Start(clusterReady)
	log.Debug("START: Cluster")
	log.Debug("Waiting on sync operation.")
	<-clusterReady
	log.Debug("Waiting done.")
	if app.Cluster.genRSA {
		err = app.Crypto.GenerateSharedKey()
		if err != nil {
			panic(err)
		}
		err = app.Crypto.SealSharedKey(app.Crypto.sharedkey, app.Crypto.privkey, false)
		if err != nil {
			panic(err)
		}
	}
	plugins, err := NewPlugins(app)
	if err != nil {
		panic(err)
	}
	app.Plugins = plugins
	TERMINATOR["plugins"] = plugins.terminate
	go app.Plugins.Start()
	kv, err := newKV(app)
	if err != nil {
		panic(err)
	}
	app.KVInit = true
	app.KV = kv
	TERMINATOR["kv"] = kv.terminate
	auth, err := NewAuth(app)
	if err != nil {
		panic(err)
	}
	app.Auth = auth
	TERMINATOR["auth"] = auth.terminate
	api, err := NewAPI(app)
	if err != nil {
		panic(err)
	}
	app.API = api
	TERMINATOR["api"] = api.terminate
	// START SHIT
	go app.KV.Start()
	log.Debug("START: KV")
	go app.API.Start()
	log.Debug("START: API")
	<-kill
	log.Warn("Got kill signal from OS, shutting down...")
	for _, t := range []string{"api", "auth", "kv", "cluster", "log", "plugins"} {
		log.Warn("Shutting down " + t)
		TERMINATOR[t] <- true
	}

}
