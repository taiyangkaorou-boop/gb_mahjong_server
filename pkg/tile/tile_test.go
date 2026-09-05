package tile

import "testing"

func TestRoundTripAll42(t *testing.T) {
	all := AllStandard()
	if len(all) != 42 {
		t.Fatalf("want 42 tiles, got %d", len(all))
	}
	seen := map[Tile]bool{}
	for _, x := range all {
		if !x.Valid() {
			t.Fatalf("invalid %v", x)
		}
		id := ToLibID(x)
		back := FromLibID(id)
		if back != x {
			t.Fatalf("%v lib=%d back=%v", x, id, back)
		}
		if seen[x] {
			t.Fatalf("dup %v", x)
		}
		seen[x] = true
	}
	if FromLibID(0) != 0 || FromLibID(99) != 0 || ToLibID(0) != 0 {
		t.Fatal("invalid ids must map to 0")
	}
}

func TestKnownEncodings(t *testing.T) {
	cases := []struct {
		t    Tile
		want byte
		lib  int
		str  string
	}{
		{Of(SuitWan, 1), 0x11, 1, "1m"},
		{Of(SuitBing, 9), 0x39, 27, "9p"},
		{East, 0x41, 28, "E"},
		{Zhong, 0x51, 32, "C"},
		{Of(SuitHua, 1), 0x61, 35, "a"},
		{Of(SuitHua, 8), 0x68, 42, "h"},
	}
	for _, c := range cases {
		if byte(c.t) != c.want {
			t.Errorf("%s encode=0x%02x want=0x%02x", c.str, byte(c.t), c.want)
		}
		if ToLibID(c.t) != c.lib {
			t.Errorf("%s lib=%d want=%d", c.str, ToLibID(c.t), c.lib)
		}
		if c.t.String() != c.str {
			t.Errorf("string=%s want=%s", c.t.String(), c.str)
		}
	}
}

func TestFullWall(t *testing.T) {
	w := FullWall()
	if len(w) != 144 {
		t.Fatalf("wall %d", len(w))
	}
	cnt := map[Tile]int{}
	for _, x := range w {
		cnt[x]++
	}
	if cnt[Of(SuitWan, 1)] != 4 || cnt[Of(SuitHua, 1)] != 1 {
		t.Fatalf("counts wan1=%d hua1=%d", cnt[Of(SuitWan, 1)], cnt[Of(SuitHua, 1)])
	}
}

func TestRemoveNAndSort(t *testing.T) {
	a := Of(SuitWan, 3)
	b := Of(SuitTiao, 1)
	in := []Tile{a, b, a, East}
	got, ok := RemoveN(in, a, 2)
	if !ok || len(got) != 2 || Count(got, a) != 0 {
		t.Fatalf("remove: %v ok=%v", got, ok)
	}
	if _, ok := RemoveN(in, a, 3); ok {
		t.Fatal("should fail when not enough tiles")
	}
	s := []Tile{East, a, b}
	Sort(s)
	if s[0] != a || s[1] != b || s[2] != East {
		t.Fatalf("sort %v", s)
	}
}

func TestSeatAndNextWind(t *testing.T) {
	if SeatWind(0, 0) != East || SeatWind(0, 1) != South || SeatWind(1, 1) != East {
		t.Fatalf("seat wind")
	}
	if NextWind(East) != South || NextWind(North) != East || NextWind(Zhong) != East {
		t.Fatalf("next wind")
	}
}

func TestPredSuccAndKinds(t *testing.T) {
	w5 := Of(SuitWan, 5)
	if w5.Pred() != Of(SuitWan, 4) || w5.Succ() != Of(SuitWan, 6) {
		t.Fatal("pred/succ")
	}
	if Of(SuitWan, 1).Pred() != 0 || Of(SuitWan, 9).Succ() != 0 || East.Pred() != 0 {
		t.Fatal("boundary pred/succ")
	}
	if !w5.IsShu() || !East.IsHonor() || !Of(SuitHua, 2).IsHua() || Tile(0).Valid() {
		t.Fatal("kind flags")
	}
	if Tile(0).String() != "?" {
		t.Fatalf("zero string %s", Tile(0).String())
	}
}
