package cluster_test

import (
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/cluster"
	"github.com/immortal-engine/immortal/internal/event"
)

func TestClusterCreateAndPeers(t *testing.T) {
	c := cluster.New("node-1", "127.0.0.1", 9000)
	if c.Self().ID != "node-1" {
		t.Error("wrong self ID")
	}
	if c.NodeCount() != 1 {
		t.Error("should count self")
	}
}

func TestClusterAddRemovePeer(t *testing.T) {
	c := cluster.New("node-1", "127.0.0.1", 9000)
	c.AddPeer("127.0.0.1", 9001)
	c.AddPeer("127.0.0.1", 9002)

	if c.NodeCount() != 3 {
		t.Errorf("expected 3 nodes, got %d", c.NodeCount())
	}
	if c.ActivePeerCount() != 2 {
		t.Errorf("expected 2 active peers, got %d", c.ActivePeerCount())
	}

	c.RemovePeer("127.0.0.1:9001")
	if c.NodeCount() != 2 {
		t.Errorf("expected 2 nodes after remove, got %d", c.NodeCount())
	}
}

func TestClusterEventSharing(t *testing.T) {
	// Node A (listener)
	nodeA := cluster.New("node-a", "127.0.0.1", 19876)
	var received *event.Event
	nodeA.OnEvent(func(e *event.Event) { received = e })
	if err := nodeA.Listen(); err != nil {
		t.Fatal(err)
	}
	defer nodeA.Stop()

	time.Sleep(50 * time.Millisecond)

	// Node B (sender)
	nodeB := cluster.New("node-b", "127.0.0.1", 19877)
	nodeB.AddPeer("127.0.0.1", 19876)

	e := event.New(event.TypeError, event.SeverityCritical, "shared crash").WithSource("api")
	errs := nodeB.BroadcastEvent(e)
	if len(errs) > 0 {
		t.Errorf("broadcast errors: %v", errs)
	}

	time.Sleep(100 * time.Millisecond)
	if received == nil {
		t.Fatal("node A should have received event")
	}
	if received.Message != "shared crash" {
		t.Errorf("wrong message: %s", received.Message)
	}
}

func TestClusterLeaderElection(t *testing.T) {
	c := cluster.New("node-b", "127.0.0.1", 9000)
	// No peers — self is leader
	if !c.IsLeader() {
		t.Error("solo node should be leader")
	}

	// Add peer with lower ID — self is no longer leader
	c.AddPeer("127.0.0.1", 9001) // ID = "127.0.0.1:9001" which is < "node-b" alphabetically
	// "127.0.0.1:9001" < "node-b" so self is NOT leader
	if c.IsLeader() {
		t.Error("should not be leader when peer has lower ID")
	}
}

func TestClusterBroadcastToDeadPeer(t *testing.T) {
	c := cluster.New("node-1", "127.0.0.1", 9000)
	c.AddPeer("127.0.0.1", 19999) // no one listening

	e := event.New(event.TypeError, event.SeverityError, "test")
	errs := c.BroadcastEvent(e)
	if len(errs) == 0 {
		t.Error("expected error for unreachable peer")
	}

	// Peer should be marked inactive
	peers := c.Peers()
	for _, p := range peers {
		if p.State != cluster.NodeInactive {
			t.Error("unreachable peer should be marked inactive")
		}
	}
}

func TestClusterPeersList(t *testing.T) {
	c := cluster.New("node-1", "127.0.0.1", 9000)
	c.AddPeer("127.0.0.1", 9001)
	c.AddPeer("127.0.0.1", 9002)

	peers := c.Peers()
	if len(peers) != 2 {
		t.Errorf("expected 2 peers, got %d", len(peers))
	}
}
