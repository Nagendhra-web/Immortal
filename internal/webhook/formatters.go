package webhook

import "time"

// ----------------- Discord WebHook Fromatter ----------------------
func FormatDiscord(title, message, severity string) map[string]interface{} {
	return map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       title,
				"description": message,
				"color":       discordColor(severity),
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			},
		},
	}
}

func discordColor(severity string) int {
	switch severity {
	case "critical":
		return 16711680
	case "warning":
		return 16753920
	case "info":
		return 3066993
	default:
		return 8421504
	}
}

// ---------------------- Slack WebHook Formatter ------------------------
func FormatSlack(title, message, severity string) map[string]interface{} {
	return map[string]interface{}{
		"text": title,
		"blocks": []map[string]interface{}{
			{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": "*" + title + "*\n" + message,
				},
			},
			{
				"type": "context",
				"elements": []map[string]interface{}{
					{
						"type": "mrkdwn",
						"text": "Severity: *" + severity + "*",
					},
				},
			},
		},
		"attachments": []map[string]interface{}{
			{
				"color": slackColor(severity),
			},
		},
	}
}

func slackColor(severity string) string {
	switch severity {
	case "critical":
		return "#FF0000"
	case "warning":
		return "#FFA500"
	case "info":
		return "#36A64F"
	default:
		return "#808080"
	}
}

// ---------------------- Teams webhook Formatter ------------------------------
func FormatTeams(title, message, severity string) map[string]interface{} {
	return map[string]interface{}{
		"@type":      "MessageCard",
		"@context":   "http://schema.org/extensions",
		"summary":    title,
		"themeColor": teamsColor(severity),
		"title":      title,
		"text":       message,
		"sections": []map[string]interface{}{
			{
				"facts": []map[string]interface{}{
					{
						"name":  "Severity",
						"value": severity,
					},
				},
			},
		},
	}
}

func teamsColor(severity string) string {
	switch severity {
	case "critical":
		return "FF0000"
	case "warning":
		return "FFA500"
	case "info":
		return "36A64F"
	default:
		return "808080"
	}
}
