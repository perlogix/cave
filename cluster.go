package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/perlin-network/noise"
	"github.com/perlin-network/noise/kademlia"
	"gopkg.in/logex.v1"
)

//Cluster type
type Cluster struct {
	config        *Config
	node          *noise.Node
	network       *kademlia.Protocol
	epoch         uint64
	events        chan Message
	peers         []noise.ID
	log           *logex.Logger
	locationTable []node
}

func newCluster(app *Bunker) (*Cluster, error) {
	config := app.Config
	c := &Cluster{
		config: config,
		log:    app.Logger,
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
	c.events = make(chan Message, config.Perf.BufferSize)
	err = c.registerHandlers()
	if err != nil {
		return c, err
	}
	return c, nil
}

func (c *Cluster) registerHandlers() error {
	c.node.Handle(func(ctx noise.HandlerContext) error {
		if ctx.IsRequest() {
			return nil
		}
		var msg Message
		err := json.Unmarshal(ctx.Data(), &msg)
		if err != nil {
			return err
		}
		c.log.Pretty(msg)
		c.events <- msg
		return nil
	})
	return nil
}

//Start starts the cluster
func (c *Cluster) Start() {
	for {
		// Wait for connection to our discovery host
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if _, err := c.node.Ping(ctx, c.config.Cluster.DiscoveryHost); err == nil {
			break
		}
		cancel()
	}
	for {
		// once we get a peer connection we can get the rest of the peers
		c.peers = c.network.Discover()
		if time.Now().Unix()%60 == 0 {
			c.locationTable = []node{}
			for _, p := range c.peers {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				start := time.Now()
				_, err := c.node.Ping(ctx, p.Address)
				if err != nil {
					cancel()
					continue
				}
				diff := time.Now().Sub(start)
				c.locationTable = append(c.locationTable, node{
					id:       p.ID.String(),
					address:  p.Address,
					distance: diff,
				})
				cancel()
			}
			c.log.Debugf("Updated location table. %+v", c.locationTable)
			time.Sleep(1 * time.Second)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Emit sends a message to the cluster
func (c *Cluster) Emit(msg Message) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	for _, p := range c.peers {
		go func(b []byte, p string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := c.node.Send(ctx, p, b)
			if err != nil {
				c.log.Error(err)
			}
		}(b, p.Address)
	}
	return nil
}
