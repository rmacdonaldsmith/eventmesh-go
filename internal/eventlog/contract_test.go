package eventlog

import (
	"testing"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/eventlog/eventlogtest"
	eventlogpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
)

func TestInMemoryEventLog_Contract(t *testing.T) {
	eventlogtest.RunContract(t, "in_memory", func(t testing.TB) eventlogpkg.EventLog {
		t.Helper()
		return NewInMemoryEventLog()
	})
}
