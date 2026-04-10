package webhook

import "testing"

// -------- DISCORD --------

func TestFormatDiscord(t *testing.T) {
	payload := FormatDiscord("Server Down", "API returning 500", "critical")

	embeds := payload["embeds"].([]map[string]interface{})
	if len(embeds) == 0 {
		t.Fatal("expected embeds")
	}

	if embeds[0]["title"] != "Server Down" {
		t.Error("wrong title")
	}

	if embeds[0]["color"] != 16711680 {
		t.Error("wrong color for critical")
	}
}

// -------- SLACK --------

func TestFormatSlack(t *testing.T) {
	payload := FormatSlack("Server Down", "API returning 500", "critical")

	if payload["text"] != "Server Down" {
		t.Error("wrong text")
	}

	attachments := payload["attachments"].([]map[string]interface{})
	if attachments[0]["color"] != "#FF0000" {
		t.Error("wrong slack color")
	}
}

// -------- TEAMS --------

func TestFormatTeams(t *testing.T) {
	payload := FormatTeams("Server Down", "API returning 500", "critical")

	if payload["@type"] != "MessageCard" {
		t.Error("invalid teams type")
	}

	if payload["themeColor"] != "FF0000" {
		t.Error("wrong teams color")
	}
}
