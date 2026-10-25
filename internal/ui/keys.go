package ui

import tea "charm.land/bubbletea/v2"

// keySeq turns key presses into logical keys, handling the two-key "gg".
type keySeq struct{ pendingG bool }

// Resolve returns the logical key for msg: "top" (gg), "bottom" (G),
// "halfdown" (ctrl+d, pgdown), "halfup" (ctrl+u, pgup), "enter", or
// msg.String() for everything else. A lone "g" returns "" and waits for the
// next key; any other key after it cancels the sequence and is resolved
// normally.
func (k *keySeq) Resolve(msg tea.KeyPressMsg) string {
	s := msg.String()
	pending := k.pendingG
	k.pendingG = false
	switch s {
	case "g":
		if pending {
			return "top"
		}
		k.pendingG = true
		return ""
	case "G":
		return "bottom"
	case "ctrl+d", "pgdown":
		return "halfdown"
	case "ctrl+u", "pgup":
		return "halfup"
	case "enter", "kpenter":
		return "enter"
	}
	return s
}
