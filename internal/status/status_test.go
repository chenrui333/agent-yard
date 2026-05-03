package status

import (
	"errors"
	"testing"

	"github.com/chenrui333/agent-yard/internal/task"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestRenderSummaryReturnsWriterError(t *testing.T) {
	err := RenderSummary(failingWriter{}, []Row{{TaskID: "route53", LedgerStatus: task.StatusReady}})
	if err == nil {
		t.Fatal("RenderSummary returned nil for failing writer")
	}
}

func TestRenderBoardReturnsWriterError(t *testing.T) {
	err := RenderBoard(failingWriter{}, []Row{{TaskID: "route53", LedgerStatus: task.StatusReady}})
	if err == nil {
		t.Fatal("RenderBoard returned nil for failing writer")
	}
}
