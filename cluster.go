package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/perlin-network/noise"
	"github.com/perlin-network/noise/kademlia"
)

//Cluster type
type Cluster struct {
	config  *Config
	node    *noise.Node
	network *kademlia.Protocol
	epoch   uint64
	events  chan ClusterEvent
	peers   []noise.ID
}

//ClusterEvent is an event
type ClusterEvent struct {
	Type string
}

//Marshal function
func (ce *ClusterEvent) Marshal() []byte {
	d, _ := json.Marshal(ce)
	return d
}

func newCluster(config *Config) (*Cluster, error) {
	c := &Cluster{
		config: config,
	}
	node, err := noise.NewNode(
		noise.WithNodeAddress(config.Cluster.AdvertiseHost),
		noise.WithNodeBindPort(config.Cluster.BindPort),
	)
	if err != nil {
		return c, err
	}
	c.node = node
	c.network = kademlia.New()
	c.node.Bind(c.network.Protocol())
	if err := c.node.Listen(); err != nil {
		return c, err
	}
	c.events = make(chan ClusterEvent, config.Perf.BufferSize)
	//err = c.registerHandlers()
	if err != nil {
		return c, err
	}
	return c, nil
}

func (c *Cluster) registerHandlers() error {
	c.node.Handle(func(ctx noise.HandlerContext) error {
		if !ctx.IsRequest() {
			return nil
		}
		var e ClusterEvent
		err := json.Unmarshal(ctx.Data(), &e)
		if err != nil {
			return err
		}
		c.events <- e
		return nil
	})
	return nil
}

//Start starts the cluster
func (c *Cluster) Start() {
	for {
		if _, err := c.node.Ping(context.TODO(), c.config.Cluster.DiscoveryHost); err != nil {
			log.Println(err)
		}
		c.peers = c.network.Discover()
		time.Sleep(1 * time.Second)
	}
}
