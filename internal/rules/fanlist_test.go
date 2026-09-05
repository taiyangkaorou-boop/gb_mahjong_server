package rules

import "testing"

func TestFanNameTable(t *testing.T) {
	if NameOf(1) != "大四喜" {
		t.Fatalf("%q", NameOf(1))
	}
	if NameOf(FanHuapai) != "花牌" {
		t.Fatalf("huapai %q", NameOf(FanHuapai))
	}
	if NameOf(82) != "明暗杠" {
		t.Fatalf("%q", NameOf(82))
	}
	if NameOf(-1) != "未知" || NameOf(999) != "未知" {
		t.Fatal("unknown")
	}
	if len(FanNames) < 83 {
		t.Fatalf("len=%d", len(FanNames))
	}
}
