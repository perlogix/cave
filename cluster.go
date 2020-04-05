package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/perlin-network/noise"
	"github.com/perlin-network/noise/kademlia"
)

//Cluster type
type Cluster struct {
	terminate     chan bool
	config        *Config
	node          *noise.Node
	network       *kademlia.Protocol
	epoch         uint64
	events        chan Message
	updates       chan Message
	sync          chan Message
	peers         []noise.ID
	log           *Log
	locationTable []node
}

func newCluster(app *Bunker) (*Cluster, error) {
	config := app.Config
	c := &Cluster{
		config:    config,
		log:       app.Logger,
		terminate: make(chan bool),
	}
	if c.config.Mode == "dev" {
		return c, nil
	}
	node, err := noise.NewNode(
		noise.WithNodeAddress(config.Cluster.AdvertiseHost),
		noise.WithNodeBindPort(config.Cluster.BindPort),
		noise.WithNodeIdleTimeout(300*time.Second),
		noise.WithNodeMaxInboundConnections(4096),
		noise.WithNodeMaxOutboundConnections(4096),
	)
	if err != nil {
		return c, err
	}
	c.node = node
	c.network = kademlia.New()
	c.node.Bind(c.network.Protocol())
	return c, nil
}

func (c *Cluster) registerHandlers(events chan Message, updates chan Message, sync chan Message) error {
	c.node.Handle(func(ctx noise.HandlerContext) error {
		if ctx.IsRequest() {
			return nil
		}
		go func() {
			var msg Message
			err := json.Unmarshal(ctx.Data(), &msg)
			if err != nil {
				return
			}
			c.log.Pretty(msg)
			switch msg.Type {
			case "event":
				c.events <- msg
			case "update":
				c.updates <- msg
			case "sync":
				c.sync <- msg
			default:
				c.log.ErrorF("No channel for message type %s", msg.Type)

			}
			c.log.Debug("UPDATES:", len(c.updates))
		}()
		return nil
	})
	return nil
}

//Start starts the cluster
func (c *Cluster) Start() {
	if err := c.node.Listen(); err != nil {
		c.log.Fatal(err)
	}
	if c.config.Mode == "dev" {
		return
	}
	c.log.Debug("Start clustering")
	for {
		c.log.Debug("waiting for peers")
		select {
		case <-c.terminate:
			return
		default:
			// Wait for connection to our discovery host
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			if _, err := c.node.Ping(ctx, c.config.Cluster.DiscoveryHost); err == nil {
				cancel()
				break
			}
			cancel()
			time.Sleep(1 * time.Second)
		}
		break
	}
	c.log.Debug("Found at least 1 peer")
	index := 0
	for {
		// once we get a peer connection we can get the rest of the peers
		select {
		case <-c.terminate:
			c.log.Info("Got termination signal")
			return
		default:
			//c.log.Debug("Discovering network")
			c.peers = c.network.Discover()
			if index == 60 || index == 0 { // every 30 seconds or so
				ltab := []node{}
				for _, p := range c.peers {
					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
					start := time.Now()
					_, err := c.node.Ping(ctx, p.Address)
					if err != nil {
						c.log.Error(err)
						cancel()
						continue
					}
					diff := time.Now().Sub(start)
					ltab = append(ltab, node{
						ID:       p.ID.String(),
						Address:  p.Address,
						Distance: diff,
					})
					cancel()
				}
				c.locationTable = ltab
				index = 0
			}
			time.Sleep(500 * time.Millisecond)
			index++
		}
	}
}

// Emit sends a message to the cluster
func (c *Cluster) Emit(typ string, data []byte, dtype string) error {
	if c.config.Mode == "dev" {
		return nil
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	msg := &Message{
		Epoch:    c.epoch + 1,
		Data:     data,
		DataType: dtype,
		Type:     typ,
		ID:       id.String(),
		Origin:   c.node.Addr(),
	}
	msg.Epoch = c.epoch + 1
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
	c.epoch++
	return nil
}
