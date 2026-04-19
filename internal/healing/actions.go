package healing

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Nagendhra-web/Immortal/internal/event"
)

func ActionExec(command string) ActionFunc {
	return func(e *event.Event) error {
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", command)
		} else {
			cmd = exec.Command("sh", "-c", command)
		}
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("command '%s' failed: %v (output: %s)", command, err, strings.TrimSpace(string(output)))
		}
		return nil
	}
}

func ActionLog(message string) ActionFunc {
	return func(e *event.Event) error {
		fmt.Printf("[immortal-heal] %s: %s\n", message, e.Message)
		return nil
	}
}

func ActionSequence(actions ...ActionFunc) ActionFunc {
	return func(e *event.Event) error {
		for _, action := range actions {
			if err := action(e); err != nil {
				return err
			}
		}
		return nil
	}
}

func ActionRetry(action ActionFunc, maxAttempts int) ActionFunc {
	return func(e *event.Event) error {
		var lastErr error
		for i := 0; i < maxAttempts; i++ {
			lastErr = action(e)
			if lastErr == nil {
				return nil
			}
		}
		return fmt.Errorf("action failed after %d attempts: %v", maxAttempts, lastErr)
	}
}
