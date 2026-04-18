package pqaudit

import (
	"bytes"
	"testing"
)

// TestRealWorld_AttackerTriesToHideAction simulates an attacker who appends a
// sensitive "restart_database" entry and then mutates it to "noop" hoping to
// erase evidence. The ledger must detect the tampering without needing the
// original data.
func TestRealWorld_AttackerTriesToHideAction(t *testing.T) {
	l := newTestLedger(t)

	// Three normal entries before the sensitive one.
	for _, action := range []string{"login", "read_config", "update_setting"} {
		if _, err := l.Append(action, "admin", "system", "", true); err != nil {
			t.Fatalf("Append %s: %v", action, err)
		}
	}

	// The sensitive entry the attacker wants to hide (Seq 4, index 3).
	if _, err := l.Append("restart_database", "admin", "db-primary", "maintenance window", true); err != nil {
		t.Fatalf("Append restart_database: %v", err)
	}

	// Two more normal entries so the chain continues after the attack target.
	for _, action := range []string{"read_logs", "logout"} {
		if _, err := l.Append(action, "admin", "system", "", true); err != nil {
			t.Fatalf("Append %s: %v", action, err)
		}
	}

	if l.Count() != 6 {
		t.Fatalf("want 6 entries, got %d", l.Count())
	}

	// Save the honest Merkle root before the attack.
	rootBefore := l.MerkleRoot()

	// Attacker mutates the Action field of the 4th entry (index 3, Seq 4).
	l.mu.Lock()
	l.entries[3].Action = "noop"
	l.mu.Unlock()

	// (a) Verify must return false.
	ok, issues := l.Verify()
	if ok {
		t.Fatal("(a) expected Verify to return false after tampering")
	}

	// (b) The tampered entry's Seq must appear in the issues list.
	found := false
	for _, iss := range issues {
		if iss.Seq == 4 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("(b) expected issue at Seq 4, got: %v", issues)
	}

	// (c) MerkleRoot after the attack must differ from the pre-attack root.
	rootAfter := l.MerkleRoot()
	if bytes.Equal(rootBefore, rootAfter) {
		t.Fatal("(c) MerkleRoot did not change after tampering — attack undetected at Merkle level")
	}
}
