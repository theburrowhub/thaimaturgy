package domain

import "testing"

func TestSplitActions(t *testing.T) {
	text := "The hall is dark and cold.\nA door creaks to the north.\n\nPosibles acciones:\n- Abrir la puerta\n- Encender una antorcha"
	narr, heading, actions := SplitActions(text)
	if heading != "Posibles acciones:" {
		t.Errorf("heading = %q", heading)
	}
	if narr != "The hall is dark and cold.\nA door creaks to the north." {
		t.Errorf("narration wrong: %q", narr)
	}
	if actions != "- Abrir la puerta\n- Encender una antorcha" {
		t.Errorf("actions wrong: %q", actions)
	}
}

func TestSplitActionsEnglishAndNoHeading(t *testing.T) {
	narr, heading, actions := SplitActions("You enter.\nPossible actions:\nRun\nHide")
	if heading != "Possible actions:" || actions != "Run\nHide" || narr != "You enter." {
		t.Errorf("english split wrong: narr=%q heading=%q actions=%q", narr, heading, actions)
	}
	// No heading → everything is narration, actions empty.
	n2, h2, a2 := SplitActions("Just a plain narration with no actions.")
	if h2 != "" || a2 != "" || n2 != "Just a plain narration with no actions." {
		t.Errorf("no-heading case wrong: %q / %q / %q", n2, h2, a2)
	}
}

func TestSplitActionsMidLineHeadingIgnored(t *testing.T) {
	// A heading not at the start of a line must NOT be treated as the marker.
	text := "The sign lists possible actions: run, hide, fight — all painted in red."
	narr, heading, actions := SplitActions(text)
	if heading != "" || actions != "" {
		t.Errorf("mid-line heading should be ignored; got heading=%q actions=%q", heading, actions)
	}
	if narr != text {
		t.Errorf("narration should be the whole text")
	}
}

func TestSplitActionsUsesLastHeading(t *testing.T) {
	// If the heading somehow appears twice, the LAST one delimits the actions.
	text := "Possible actions: (draft)\nintro\n\nPossible actions:\nreal one"
	_, _, actions := SplitActions(text)
	if actions != "real one" {
		t.Errorf("should split at the last heading; got %q", actions)
	}
}
