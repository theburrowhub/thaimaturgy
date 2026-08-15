package rulesystem

import "testing"

func TestEvalFormulaBasic(t *testing.T) {
	val, err := EvalFormula("2 + 3 * 4", nil)
	if err != nil {
		t.Fatal(err)
	}
	if val != 14 {
		t.Fatalf("got %v want 14", val)
	}
}

func TestEvalFormulaVariables(t *testing.T) {
	val, err := EvalFormula("hit_die_max + con_mod + (level-1)*4", map[string]float64{
		"hit_die_max": 10, "con_mod": 2, "level": 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if val != 20 {
		t.Fatalf("got %v want 20", val)
	}
}

func TestEvalFormulaFloor(t *testing.T) {
	val, err := EvalFormula("2 + floor((level-1)/4)", map[string]float64{"level": 9})
	if err != nil {
		t.Fatal(err)
	}
	if val != 4 {
		t.Fatalf("got %v want 4", val)
	}
}

func TestEvalFormulaParens(t *testing.T) {
	val, err := EvalFormula("(con + siz) / 10", map[string]float64{"con": 55, "siz": 65})
	if err != nil {
		t.Fatal(err)
	}
	if val != 12 {
		t.Fatalf("got %v want 12", val)
	}
}

func TestEvalFormulaErrors(t *testing.T) {
	if _, err := EvalFormula("", nil); err == nil {
		t.Fatal("expected error for empty")
	}
	if _, err := EvalFormula("x + 1", nil); err == nil {
		t.Fatal("expected unknown variable error")
	}
	if _, err := EvalFormula("1/0", nil); err == nil {
		t.Fatal("expected division by zero")
	}
}
