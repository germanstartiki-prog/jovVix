package v1

import (
	"errors"
	"github.com/Improwised/jovvix/api/constants"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"testing"
)

func TestSessionActivationErrorClassification(t *testing.T) {
	for _, message := range []string{constants.ErrSessionWasCompleted, "database unavailable", constants.Unauthenticated} {
		core, logs := observer.New(zap.DebugLevel)
		terminal := logSessionActivationError(zap.New(core), "session-123", errors.New(message))
		expected := message == constants.ErrSessionWasCompleted
		if terminal != expected {
			t.Fatalf("incorrect terminal classification for %s", message)
		}
		level := zap.ErrorLevel
		if expected {
			level = zap.InfoLevel
		}
		if logs.Len() != 1 || logs.All()[0].Level != level || logs.All()[0].ContextMap()["session_id"] != "session-123" {
			t.Fatal("incorrect activation log", logs.All())
		}
	}
}
