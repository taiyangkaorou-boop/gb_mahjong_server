package settle

import "testing"

func TestPayZimo(t *testing.T) {
	s := PayHu(true, 12, 0, -1)
	if s[0] != 60 || s[1] != -20 || s[2] != -20 || s[3] != -20 {
		t.Fatalf("%v", s)
	}
}

func TestPayRon(t *testing.T) {
	s := PayHu(false, 12, 1, 0)
	// winner 1, discarder 0 pays 20, others pay 8; winner gets 20+8+8=36
	if s[0] != -20 || s[1] != 36 || s[2] != -8 || s[3] != -8 {
		t.Fatalf("%v", s)
	}
}

func TestPayWrong(t *testing.T) {
	s := PayWrong(2)
	if s[2] != -30 || s[0] != 10 || s[1] != 10 || s[3] != 10 {
		t.Fatalf("%v", s)
	}
}

func TestHuangZero(t *testing.T) {
	r := Huang()
	if r.Kind != KindHuang || r.Winner != -1 {
		t.Fatalf("%+v", r)
	}
	if r.Scores != [4]int32{} {
		t.Fatalf("scores %v", r.Scores)
	}
}

func TestPayHuBadWinner(t *testing.T) {
	s := PayHu(true, 8, -1, 0)
	if s != [4]int32{} {
		t.Fatalf("%v", s)
	}
}
