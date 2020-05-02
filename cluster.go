package main

import (
	"fmt"

	"github.com/perlin-network/noise"
)

//Cluster type
type Cluster struct {
	app           *Bunker
	terminate     chan bool
	config        *Config
	node          *noise.Node
	network       *Network
	epoch         uint64
	updates       chan Message
	synced        chan bool
	peers         map[string]Peer
	log           *Log
	locationTable []node
	genRSA        bool
	metrics       map[string]interface{}
	advertiseHost string
}

func newCluster(app *Bunker) (*Cluster, error) {
	config := app.Config
	c := &Cluster{
		app:           app,
		config:        config,
		log:           app.Logger,
		terminate:     make(chan bool),
		synced:        make(chan bool),
		genRSA:        false,
		advertiseHost: fmt.Sprintf("%s:%v", config.Cluster.Host, config.Cluster.Port),
	}
	if c.config.Mode == "dev" {
		c.genRSA = true
		return c, nil
	}
	net, err := NewNetwork(app, NetworkOptions{
		AnnounceHost:   c.config.Cluster.DiscoveryHost,
		Port:           c.config.Cluster.Port,
		IP:             c.config.Cluster.Host,
		MaxMessageSize: 99999,
		LogFunc:        c.log.print,
		App:            app,
		Certificate:    config.Cluster.Certificate,
		CertificateKey: config.Cluster.CertificateKey,
	})
	if err != nil {
		return c, err
	}
	c.network = net
	c.peers = net.Node.Peers
	err = c.network.Start()
	if err != nil {
		return c, err
	}
	if len(c.peers) == 1 {
		c.genRSA = true
	}
	return c, nil
}

// Start function
func (c *Cluster) Start(ready chan bool) error {
	ready <- true
	return nil
}
