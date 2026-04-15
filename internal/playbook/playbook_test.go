package playbook_test

import (
	"errors"
	"testing"

	"github.com/immortal-engine/immortal/internal/playbook"
)

func TestNew(t *testing.T) {
	r := playbook.New()
	if r == nil {
		t.Fatal("expected runner")
	}
}

func TestRegister(t *testing.T) {
	r := playbook.New()
	r.Register("test-playbook", []playbook.Step{
		{Name: "step-1", Action: func() error { return nil }},
	})

	pb := r.Get("test-playbook")
	if pb == nil {
		t.Fatal("expected playbook")
	}
	if pb.Name != "test-playbook" {
		t.Errorf("expected test-playbook, got %s", pb.Name)
	}
}

func TestRunSuccess(t *testing.T) {
	r := playbook.New()
	order := []string{}

	r.Register("deploy", []playbook.Step{
		{Name: "backup", Action: func() error { order = append(order, "backup"); return nil }},
		{Name: "migrate", Action: func() error { order = append(order, "migrate"); return nil }},
		{Name: "restart", Action: func() error { order = append(order, "restart"); return nil }},
	})

	exec, err := r.Run("deploy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.Status != "success" {
		t.Errorf("expected success, got %s", exec.Status)
	}
	if len(exec.Results) != 3 {
		t.Errorf("expected 3 results, got %d", len(exec.Results))
	}
	if len(order) != 3 || order[0] != "backup" || order[1] != "migrate" || order[2] != "restart" {
		t.Errorf("unexpected order: %v", order)
	}
}

func TestRunFailureWithRollback(t *testing.T) {
	rolledBack := []string{}

	r := playbook.New()
	r.Register("deploy", []playbook.Step{
		{
			Name:     "step-1",
			Action:   func() error { return nil },
			Rollback: func() error { rolledBack = append(rolledBack, "step-1"); return nil },
		},
		{
			Name:     "step-2",
			Action:   func() error { return errors.New("migration failed") },
			Rollback: func() error { rolledBack = append(rolledBack, "step-2"); return nil },
		},
	})

	exec, err := r.Run("deploy")
	if err == nil {
		t.Fatal("expected error")
	}
	if exec.Status != "failed" {
		t.Errorf("expected failed, got %s", exec.Status)
	}

	// step-1 should be rolled back
	if len(rolledBack) != 1 || rolledBack[0] != "step-1" {
		t.Errorf("expected step-1 rolled back, got %v", rolledBack)
	}

	// Check result statuses
	for _, res := range exec.Results {
		if res.Name == "step-1" && res.Status != "rolled_back" {
			t.Errorf("expected step-1 rolled_back, got %s", res.Status)
		}
		if res.Name == "step-2" && res.Status != "failed" {
			t.Errorf("expected step-2 failed, got %s", res.Status)
		}
	}
}

func TestRunWithCondition(t *testing.T) {
	r := playbook.New()
	executed := false

	r.Register("conditional", []playbook.Step{
		{
			Name:      "skipped-step",
			Action:    func() error { executed = true; return nil },
			Condition: func() bool { return false },
		},
		{
			Name:   "normal-step",
			Action: func() error { return nil },
		},
	})

	exec, err := r.Run("conditional")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if executed {
		t.Error("skipped step should not have executed")
	}
	if exec.Results[0].Status != "skipped" {
		t.Errorf("expected skipped, got %s", exec.Results[0].Status)
	}
	if exec.Results[1].Status != "success" {
		t.Errorf("expected success, got %s", exec.Results[1].Status)
	}
}

func TestRunWithRetries(t *testing.T) {
	attempts := 0

	r := playbook.New()
	r.Register("retry-test", []playbook.Step{
		{
			Name: "flaky-step",
			Action: func() error {
				attempts++
				if attempts < 3 {
					return errors.New("not ready")
				}
				return nil
			},
			Retries: 5,
		},
	})

	exec, err := r.Run("retry-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.Status != "success" {
		t.Errorf("expected success, got %s", exec.Status)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if exec.Results[0].Attempt != 3 {
		t.Errorf("expected attempt 3, got %d", exec.Results[0].Attempt)
	}
}

func TestDryRun(t *testing.T) {
	executed := false

	r := playbook.New()
	r.Register("test", []playbook.Step{
		{Name: "step-1", Action: func() error { executed = true; return nil }},
		{Name: "step-2", Action: func() error { return nil }, Condition: func() bool { return false }},
	})

	exec, err := r.DryRun("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if executed {
		t.Error("dry run should not execute actions")
	}
	if exec.Status != "dry_run" {
		t.Errorf("expected dry_run, got %s", exec.Status)
	}
	if exec.Results[0].Status != "success" {
		t.Errorf("expected success for step-1, got %s", exec.Results[0].Status)
	}
	if exec.Results[1].Status != "skipped" {
		t.Errorf("expected skipped for step-2, got %s", exec.Results[1].Status)
	}
}

func TestHistory(t *testing.T) {
	r := playbook.New()
	r.Register("test", []playbook.Step{
		{Name: "step", Action: func() error { return nil }},
	})

	r.Run("test")
	r.Run("test")

	history := r.History()
	if len(history) != 2 {
		t.Errorf("expected 2 executions, got %d", len(history))
	}
}

func TestList(t *testing.T) {
	r := playbook.New()
	r.Register("alpha", []playbook.Step{{Name: "s", Action: func() error { return nil }}})
	r.Register("beta", []playbook.Step{{Name: "s", Action: func() error { return nil }}})

	list := r.List()
	if len(list) != 2 {
		t.Errorf("expected 2, got %d", len(list))
	}
}

func TestGet(t *testing.T) {
	r := playbook.New()
	r.Register("test", []playbook.Step{{Name: "s", Action: func() error { return nil }}})

	if r.Get("test") == nil {
		t.Error("expected playbook")
	}
	if r.Get("nonexistent") != nil {
		t.Error("expected nil for nonexistent")
	}
}

func TestLastExecution(t *testing.T) {
	r := playbook.New()
	r.Register("test", []playbook.Step{
		{Name: "step", Action: func() error { return nil }},
	})

	r.Run("test")
	r.Run("test")

	last := r.LastExecution("test")
	if last == nil {
		t.Fatal("expected last execution")
	}

	none := r.LastExecution("nonexistent")
	if none != nil {
		t.Error("expected nil for nonexistent")
	}
}

func TestRunNonexistent(t *testing.T) {
	r := playbook.New()
	_, err := r.Run("does-not-exist")
	if err == nil {
		t.Error("expected error for nonexistent playbook")
	}
}
