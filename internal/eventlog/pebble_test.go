package eventlog

import (
	"testing"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/eventlog/eventlogtest"
	eventlogpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
)

func TestPebbleEventLog_Contract(t *testing.T) {
	eventlogtest.RunContract(t, "pebble", func(t testing.TB) eventlogpkg.EventLog {
		t.Helper()

		log, err := NewPebbleEventLog(t.TempDir())
		if err != nil {
			t.Fatalf("NewPebbleEventLog failed: %v", err)
		}
		return log
	})
}

func TestPebbleEventLog_DurabilityContract(t *testing.T) {
	eventlogtest.RunDurabilityContract(t, "pebble", func(t testing.TB) (eventlogpkg.EventLog, func(testing.TB) eventlogpkg.EventLog) {
		t.Helper()

		dir := t.TempDir()
		open := func(t testing.TB) eventlogpkg.EventLog {
			t.Helper()

			log, err := NewPebbleEventLog(dir)
			if err != nil {
				t.Fatalf("NewPebbleEventLog failed: %v", err)
			}
			return log
		}

		return open(t), open
	})
}
