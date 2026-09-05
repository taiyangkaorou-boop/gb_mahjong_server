package logx

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func withCapture(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	oldLvl := CurrentLevel()
	oldExit := exitFunc
	SetOutput(&buf)
	t.Cleanup(func() {
		SetLevel(oldLvl)
		SetOutput(os.Stderr)
		exitFunc = oldExit
	})
	return &buf
}

func TestParseLevel(t *testing.T) {
	if ParseLevel("TRACE") != LevelTrace || ParseLevel("warning") != LevelWarn {
		t.Fatal("parse")
	}
	if ParseLevel("nope") != LevelInfo || ParseLevel("") != LevelInfo {
		t.Fatal("default info")
	}
	if KnownLevel("nope") || !KnownLevel("error") || !KnownLevel("") {
		t.Fatal("known")
	}
}

func TestLevelFilter(t *testing.T) {
	buf := withCapture(t)
	SetLevel(LevelWarn)
	Tracef("t")
	Infof("i")
	Warnf("w %d", 1)
	Errorf("e")
	s := buf.String()
	if strings.Contains(s, "[TRACE]") || strings.Contains(s, "[INFO]") {
		t.Fatalf("leaked low level: %s", s)
	}
	if !strings.Contains(s, "[WARN]") || !strings.Contains(s, "w 1") {
		t.Fatalf("missing warn: %s", s)
	}
	if !strings.Contains(s, "[ERROR]") {
		t.Fatalf("missing error: %s", s)
	}
}

func TestRecover(t *testing.T) {
	buf := withCapture(t)
	SetLevel(LevelInfo)
	func() {
		defer Recover("unit")
		panic("boom")
	}()
	s := buf.String()
	if !strings.Contains(s, "[ERROR]") || !strings.Contains(s, "where=unit") || !strings.Contains(s, "boom") {
		t.Fatalf("recover log: %s", s)
	}
}

func TestFatalf(t *testing.T) {
	buf := withCapture(t)
	code := 0
	SetExit(func(c int) { code = c })
	Fatalf("dead %s", "now")
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(buf.String(), "[FATAL]") || !strings.Contains(buf.String(), "dead now") {
		t.Fatalf("fatal log: %s", buf.String())
	}
}
