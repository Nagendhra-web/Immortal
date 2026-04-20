// Package chatops exposes Immortal via Slack and Discord surfaces.
//
// The package is deliberately SDK-free. It parses command strings,
// dispatches to a pluggable Handler, and formats the reply as either
// Slack Block Kit JSON or Discord embed JSON. Webhook receivers in your
// operator integration can decode the incoming JSON, call Parse() +
// Dispatch(), and serialize the reply with Slack() or Discord().
//
// Supported commands (case-insensitive, whitespace-flexible):
//
//	/immortal status                    engine + intents summary
//	/immortal incidents                 active incidents with Verdicts
//	/immortal explain <id>              narrator Verdict for one incident
//	/immortal suggest <service>         evolve suggestions for that service
//	/immortal pause | resume            toggle autonomous healing
//	/immortal help                      list commands
//
// Each invocation is auto-logged to the audit channel via the Auditor
// interface so every chat command leaves a signed receipt.
package chatops

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Command is a parsed chat instruction.
type Command struct {
	Name string   // "status", "incidents", "explain", "suggest", "pause", "resume", "help"
	Args []string // trailing positional arguments
	Raw  string   // original text for audit logs
}

// Reply is the result of a Handler. The Markdown field is used as the
// fallback if no rich formatting is requested.
type Reply struct {
	Title    string       `json:"title"`
	Markdown string       `json:"markdown"`     // fallback plaintext + md
	Fields   []ReplyField `json:"fields,omitempty"` // optional key/value sections
	Severity Severity     `json:"severity"`     // drives color: ok / warn / err / info
	Actions  []ReplyAction `json:"actions,omitempty"` // optional buttons
}

// ReplyField is one row in the reply (e.g. "Active incidents: 3").
type ReplyField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// ReplyAction is a button that posts back to the engine.
type ReplyAction struct {
	Label  string `json:"label"`
	Command string `json:"command"` // e.g. "/immortal explain inc-42"
	Style   string `json:"style"`   // "primary" | "danger" | "default"
}

// Severity drives Slack attachment colors and Discord embed colors.
type Severity string

const (
	SeverityInfo Severity = "info"
	SeverityOK   Severity = "ok"
	SeverityWarn Severity = "warn"
	SeverityErr  Severity = "err"
)

// Auditor records the command for later verification. Typically wired
// to the PQ audit chain via a tiny adapter in the caller.
type Auditor interface {
	Record(user, channel, raw string, reply Reply) error
}

// Handler is the dependency set used by Dispatch. Providing a concrete
// Handler turns a chat invocation into a real interaction with the
// engine. Every field is optional; unset means the matching command
// returns a "not configured" reply.
type Handler struct {
	Status         func() Reply                // /status
	Incidents      func() Reply                // /incidents
	Explain        func(id string) Reply       // /explain <id>
	Suggest        func(service string) Reply  // /suggest <service>
	Pause          func() Reply                // /pause
	Resume         func() Reply                // /resume
	Help           func() Reply                // /help (auto-generated if nil)
	Audit          Auditor                     // optional
}

// Parse turns "/immortal explain inc-42" or "immortal   explain  inc-42"
// into a Command. Returns a non-nil error only on empty input.
func Parse(text string) (Command, error) {
	fields := strings.Fields(strings.TrimSpace(text))
	// Strip leading "/immortal" or "immortal" mention.
	for len(fields) > 0 && (strings.EqualFold(fields[0], "/immortal") || strings.EqualFold(fields[0], "immortal")) {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		return Command{}, errors.New("empty command")
	}
	cmd := Command{
		Name: strings.ToLower(fields[0]),
		Args: fields[1:],
		Raw:  strings.TrimSpace(text),
	}
	return cmd, nil
}

// Dispatch runs the parsed command against a Handler and returns the Reply.
// Records the command via h.Audit if configured. Unknown commands return
// the Help reply; this means wrong input never looks like an error to the
// user.
func Dispatch(cmd Command, h Handler, user, channel string) Reply {
	reply := runCommand(cmd, h)
	if h.Audit != nil {
		// Audit failures do not abort the reply; they are logged best-effort.
		_ = h.Audit.Record(user, channel, cmd.Raw, reply)
	}
	return reply
}

func runCommand(cmd Command, h Handler) Reply {
	switch cmd.Name {
	case "status":
		if h.Status != nil {
			return h.Status()
		}
	case "incidents":
		if h.Incidents != nil {
			return h.Incidents()
		}
	case "explain":
		if len(cmd.Args) == 0 {
			return Reply{Title: "explain requires an incident id", Markdown: "Usage: `/immortal explain <incident_id>`", Severity: SeverityWarn}
		}
		if h.Explain != nil {
			return h.Explain(cmd.Args[0])
		}
	case "suggest":
		if len(cmd.Args) == 0 {
			return Reply{Title: "suggest requires a service", Markdown: "Usage: `/immortal suggest <service>`", Severity: SeverityWarn}
		}
		if h.Suggest != nil {
			return h.Suggest(cmd.Args[0])
		}
	case "pause":
		if h.Pause != nil {
			return h.Pause()
		}
	case "resume":
		if h.Resume != nil {
			return h.Resume()
		}
	case "help":
		if h.Help != nil {
			return h.Help()
		}
		return defaultHelpReply()
	default:
		if h.Help != nil {
			r := h.Help()
			r.Title = "Unknown command: " + cmd.Name
			r.Severity = SeverityWarn
			return r
		}
		return defaultHelpReply()
	}
	// Handler field was nil for the matched command.
	return Reply{
		Title:    "Command not configured",
		Markdown: fmt.Sprintf("`%s` is not wired up on this deployment.", cmd.Name),
		Severity: SeverityWarn,
	}
}

func defaultHelpReply() Reply {
	return Reply{
		Title: "Immortal commands",
		Markdown: strings.Join([]string{
			"`/immortal status`                engine + intents summary",
			"`/immortal incidents`             active incidents",
			"`/immortal explain <id>`          narrator Verdict for an incident",
			"`/immortal suggest <service>`     evolve suggestions",
			"`/immortal pause`                 stop autonomous healing",
			"`/immortal resume`                restart autonomous healing",
			"`/immortal help`                  this message",
		}, "\n"),
		Severity: SeverityInfo,
	}
}

// ── Slack formatter (Block Kit JSON) ─────────────────────────────────────

// Slack renders the Reply as a Slack Block Kit JSON payload. Returns
// ready-to-POST bytes for the `response_url` returned by Slack slash
// commands.
func Slack(r Reply) []byte {
	color := slackColor(r.Severity)
	blocks := []map[string]any{}
	if r.Title != "" {
		blocks = append(blocks, map[string]any{
			"type": "header",
			"text": map[string]any{"type": "plain_text", "text": r.Title},
		})
	}
	if r.Markdown != "" {
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": map[string]any{"type": "mrkdwn", "text": r.Markdown},
		})
	}
	if len(r.Fields) > 0 {
		fieldBlocks := []map[string]any{}
		for _, f := range r.Fields {
			fieldBlocks = append(fieldBlocks, map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*%s*\n%s", f.Name, f.Value),
			})
		}
		// Slack sections allow up to 10 fields; chunk if larger.
		for i := 0; i < len(fieldBlocks); i += 10 {
			end := i + 10
			if end > len(fieldBlocks) {
				end = len(fieldBlocks)
			}
			blocks = append(blocks, map[string]any{
				"type":   "section",
				"fields": fieldBlocks[i:end],
			})
		}
	}
	if len(r.Actions) > 0 {
		els := []map[string]any{}
		for _, a := range r.Actions {
			els = append(els, map[string]any{
				"type":     "button",
				"text":     map[string]any{"type": "plain_text", "text": a.Label},
				"value":    a.Command,
				"style":    a.Style,
				"action_id": "immortal:" + strings.ReplaceAll(a.Command, " ", "_"),
			})
		}
		blocks = append(blocks, map[string]any{"type": "actions", "elements": els})
	}
	payload := map[string]any{
		"response_type": "in_channel",
		"attachments": []map[string]any{
			{"color": color, "blocks": blocks},
		},
	}
	out, _ := json.Marshal(payload)
	return out
}

func slackColor(s Severity) string {
	switch s {
	case SeverityOK:
		return "#22c55e"
	case SeverityWarn:
		return "#eab308"
	case SeverityErr:
		return "#ef4444"
	default:
		return "#3b82f6"
	}
}

// ── Discord formatter (webhook embed JSON) ───────────────────────────────

// Discord renders the Reply as a Discord webhook embed payload.
func Discord(r Reply) []byte {
	fields := []map[string]any{}
	for _, f := range r.Fields {
		fields = append(fields, map[string]any{
			"name":   f.Name,
			"value":  f.Value,
			"inline": f.Inline,
		})
	}
	embed := map[string]any{
		"title":       r.Title,
		"description": r.Markdown,
		"color":       discordColor(r.Severity),
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}
	if len(fields) > 0 {
		embed["fields"] = fields
	}
	payload := map[string]any{
		"embeds": []map[string]any{embed},
	}
	if len(r.Actions) > 0 {
		// Discord supports action rows with up to 5 buttons each.
		rows := []map[string]any{}
		current := []map[string]any{}
		for _, a := range r.Actions {
			current = append(current, map[string]any{
				"type":      2, // button
				"label":     a.Label,
				"style":     discordButtonStyle(a.Style),
				"custom_id": "immortal:" + strings.ReplaceAll(a.Command, " ", "_"),
			})
			if len(current) == 5 {
				rows = append(rows, map[string]any{"type": 1, "components": current})
				current = []map[string]any{}
			}
		}
		if len(current) > 0 {
			rows = append(rows, map[string]any{"type": 1, "components": current})
		}
		payload["components"] = rows
	}
	out, _ := json.Marshal(payload)
	return out
}

func discordColor(s Severity) int {
	// Discord colors are 24-bit RGB integers.
	switch s {
	case SeverityOK:
		return 0x22c55e
	case SeverityWarn:
		return 0xeab308
	case SeverityErr:
		return 0xef4444
	default:
		return 0x3b82f6
	}
}

func discordButtonStyle(s string) int {
	switch s {
	case "primary":
		return 1 // blurple
	case "danger":
		return 4 // red
	default:
		return 2 // gray
	}
}
