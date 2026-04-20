package chatops

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParse_StripsMentionAndSlash(t *testing.T) {
	cases := []struct {
		in       string
		wantName string
		wantArgs []string
	}{
		{"/immortal status", "status", nil},
		{"immortal incidents", "incidents", nil},
		{"/immortal explain inc-42", "explain", []string{"inc-42"}},
		{"  IMMORTAL  suggest   checkout  ", "suggest", []string{"checkout"}},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			cmd, err := Parse(c.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd.Name != c.wantName {
				t.Errorf("name: got %q want %q", cmd.Name, c.wantName)
			}
			if len(cmd.Args) != len(c.wantArgs) {
				t.Fatalf("args len: got %d want %d", len(cmd.Args), len(c.wantArgs))
			}
			for i, a := range c.wantArgs {
				if cmd.Args[i] != a {
					t.Errorf("arg[%d]: got %q want %q", i, cmd.Args[i], a)
				}
			}
		})
	}
}

func TestParse_Empty(t *testing.T) {
	if _, err := Parse(""); err == nil {
		t.Errorf("empty input should error")
	}
}

func TestDispatch_UnconfiguredHandlerReturnsSafeMessage(t *testing.T) {
	cmd, _ := Parse("/immortal status")
	r := Dispatch(cmd, Handler{}, "u", "c")
	if r.Severity != SeverityWarn {
		t.Errorf("unconfigured handler should warn; got %v", r.Severity)
	}
	if !strings.Contains(r.Markdown, "not wired") {
		t.Errorf("reply should explain the gap; got %q", r.Markdown)
	}
}

func TestDispatch_ExplainRequiresArg(t *testing.T) {
	cmd, _ := Parse("/immortal explain")
	r := Dispatch(cmd, Handler{Explain: func(string) Reply { return Reply{Title: "never called"} }}, "u", "c")
	if !strings.Contains(r.Title, "requires an incident id") {
		t.Errorf("missing arg prompt; got %q", r.Title)
	}
}

func TestDispatch_Routing(t *testing.T) {
	called := ""
	h := Handler{
		Status:    func() Reply { called = "status"; return Reply{Title: "S"} },
		Incidents: func() Reply { called = "incidents"; return Reply{Title: "I"} },
		Explain:   func(id string) Reply { called = "explain:" + id; return Reply{Title: "E"} },
		Suggest:   func(svc string) Reply { called = "suggest:" + svc; return Reply{Title: "R"} },
		Pause:     func() Reply { called = "pause"; return Reply{Title: "P"} },
		Resume:    func() Reply { called = "resume"; return Reply{Title: "R2"} },
	}
	cases := []struct {
		in   string
		want string
	}{
		{"/immortal status", "status"},
		{"/immortal incidents", "incidents"},
		{"/immortal explain inc-42", "explain:inc-42"},
		{"/immortal suggest payments", "suggest:payments"},
		{"/immortal pause", "pause"},
		{"/immortal resume", "resume"},
	}
	for _, c := range cases {
		called = ""
		cmd, _ := Parse(c.in)
		Dispatch(cmd, h, "u", "c")
		if called != c.want {
			t.Errorf("%q dispatched to %q, want %q", c.in, called, c.want)
		}
	}
}

func TestDispatch_Auditor(t *testing.T) {
	records := []string{}
	h := Handler{
		Status: func() Reply { return Reply{Title: "ok"} },
		Audit: funcAuditor(func(user, channel, raw string, reply Reply) error {
			records = append(records, raw)
			return nil
		}),
	}
	cmd, _ := Parse("/immortal status")
	Dispatch(cmd, h, "alice", "#incident-room")
	if len(records) != 1 || !strings.Contains(records[0], "status") {
		t.Errorf("auditor should record the raw command; got %v", records)
	}
}

func TestSlack_Formatting(t *testing.T) {
	r := Reply{
		Title:    "Active incidents",
		Markdown: "3 incidents · 2 critical",
		Fields:   []ReplyField{{Name: "inc-42", Value: "checkout p99 310ms"}},
		Severity: SeverityErr,
	}
	var body map[string]any
	if err := json.Unmarshal(Slack(r), &body); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	atts := body["attachments"].([]any)
	if len(atts) == 0 {
		t.Fatal("attachments missing")
	}
	if atts[0].(map[string]any)["color"] != "#ef4444" {
		t.Errorf("critical color should be red; got %v", atts[0].(map[string]any)["color"])
	}
}

func TestDiscord_Formatting(t *testing.T) {
	r := Reply{
		Title:    "Engine status",
		Markdown: "healthy",
		Severity: SeverityOK,
	}
	var body map[string]any
	if err := json.Unmarshal(Discord(r), &body); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	embeds := body["embeds"].([]any)
	if len(embeds) == 0 {
		t.Fatal("embeds missing")
	}
	color := embeds[0].(map[string]any)["color"].(float64)
	if int(color) != 0x22c55e {
		t.Errorf("ok color should be green; got %x", int(color))
	}
}

func TestDiscord_ActionsChunkAt5(t *testing.T) {
	acts := []ReplyAction{}
	for i := 0; i < 7; i++ {
		acts = append(acts, ReplyAction{Label: "a", Command: "/immortal status"})
	}
	r := Reply{Actions: acts}
	var body map[string]any
	_ = json.Unmarshal(Discord(r), &body)
	rows := body["components"].([]any)
	if len(rows) != 2 {
		t.Errorf("7 actions should produce 2 rows (5+2); got %d rows", len(rows))
	}
}

func TestDefaultHelpReply_HasAllCommands(t *testing.T) {
	r := defaultHelpReply()
	for _, cmd := range []string{"status", "incidents", "explain", "suggest", "pause", "resume"} {
		if !strings.Contains(r.Markdown, cmd) {
			t.Errorf("help missing %q", cmd)
		}
	}
}

type funcAuditor func(user, channel, raw string, reply Reply) error

func (f funcAuditor) Record(user, channel, raw string, reply Reply) error {
	return f(user, channel, raw, reply)
}
