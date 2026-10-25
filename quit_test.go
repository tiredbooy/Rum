package main

import "testing"

func TestQuitPrompt(t *testing.T) {
	if got := quitPrompt(1); got != "1 download is running. Quit and pause it?" {
		t.Fatalf("singular: %q", got)
	}
	if got := quitPrompt(3); got != "3 downloads are running. Quit and pause them?" {
		t.Fatalf("plural: %q", got)
	}
}
