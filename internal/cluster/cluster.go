package cluster

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/event"
)

type NodeState string

const (
	NodeActive   NodeState = "active"
	NodeInactive NodeState = "inactive"
	NodeDraining NodeState = "draining"
)

type Node struct {
	ID         string    `json:"id"`
	Address    string    `json:"address"`
	Port       int       `json:"port"`
	State      NodeState `json:"state"`
	LastSeen   time.Time `json:"last_seen"`
	EventCount int64     `json:"event_count"`
	HealCount  int64     `json:"heal_count"`
	Version    string    `json:"version"`
}

type Cluster struct {
	mu       sync.RWMutex
	self     *Node
	peers    map[string]*Node
	listener net.Listener
	onEvent  func(*event.Event)
	done     chan struct{}
}

func New(id, address string, port int) *Cluster {
	return &Cluster{
		self: &Node{
			ID:      id,
			Address: address,
			Port:    port,
			State:   NodeActive,
		},
		peers: make(map[string]*Node),
		done:  make(chan struct{}),
	}
}

func (c *Cluster) OnEvent(fn func(*event.Event)) {
	c.onEvent = fn
}

func (c *Cluster) Self() Node {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return *c.self
}

func (c *Cluster) AddPeer(address string, port int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := fmt.Sprintf("%s:%d", address, port)
	c.peers[id] = &Node{
		ID:       id,
		Address:  address,
		Port:     port,
		State:    NodeActive,
		LastSeen: time.Now(),
	}
	return nil
}

func (c *Cluster) RemovePeer(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.peers, id)
}

func (c *Cluster) Peers() []Node {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var result []Node
	for _, p := range c.peers {
		result = append(result, *p)
	}
	return result
}

func (c *Cluster) ActivePeerCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	count := 0
	for _, p := range c.peers {
		if p.State == NodeActive {
			count++
		}
	}
	return count
}

func (c *Cluster) BroadcastEvent(e *event.Event) []error {
	c.mu.RLock()
	peers := make([]*Node, 0, len(c.peers))
	for _, p := range c.peers {
		if p.State == NodeActive {
			peers = append(peers, p)
		}
	}
	c.mu.RUnlock()

	var errs []error
	for _, peer := range peers {
		if err := c.sendEvent(peer, e); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (c *Cluster) sendEvent(peer *Node, e *event.Event) error {
	addr := fmt.Sprintf("%s:%d", peer.Address, peer.Port)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		c.mu.Lock()
		peer.State = NodeInactive
		c.mu.Unlock()
		return fmt.Errorf("peer %s unreachable: %w", addr, err)
	}
	defer conn.Close()
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	return json.NewEncoder(conn).Encode(e)
}

func (c *Cluster) Listen() error {
	addr := fmt.Sprintf("%s:%d", c.self.Address, c.self.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("cluster listen: %w", err)
	}
	c.listener = ln

	go func() {
		for {
			select {
			case <-c.done:
				return
			default:
				conn, err := ln.Accept()
				if err != nil {
					continue
				}
				go c.handleConn(conn)
			}
		}
	}()

	return nil
}

func (c *Cluster) handleConn(conn net.Conn) {
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var e event.Event
	if err := json.NewDecoder(conn).Decode(&e); err != nil {
		return
	}
	if c.onEvent != nil {
		c.onEvent(&e)
	}
}

func (c *Cluster) Stop() {
	close(c.done)
	if c.listener != nil {
		c.listener.Close()
	}
}

func (c *Cluster) IsLeader() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, p := range c.peers {
		if p.State == NodeActive && p.ID < c.self.ID {
			return false
		}
	}
	return true
}

func (c *Cluster) NodeCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.peers) + 1
}
