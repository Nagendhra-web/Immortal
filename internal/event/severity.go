package event

// Severity represents the severity level of an event.
type Severity string

const (
	SeverityDebug    Severity = "debug"
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
	SeverityFatal    Severity = "fatal"
)

func (s Severity) Level() int {
	switch s {
	case SeverityDebug:
		return 0
	case SeverityInfo:
		return 1
	case SeverityWarning:
		return 2
	case SeverityError:
		return 3
	case SeverityCritical:
		return 4
	case SeverityFatal:
		return 5
	default:
		return -1
	}
}
