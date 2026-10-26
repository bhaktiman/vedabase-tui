package ui

import tea "charm.land/bubbletea/v2"

// maxHistory is how many places the back stack keeps.
const maxHistory = 100

// place is somewhere in the text to come back to.
type place struct {
	ref    string
	offset int
}

// here is where the reader is now. A document still loading keeps the offset
// it will be scrolled to.
func (a *App) here() place {
	if a.restoreRef == a.ref {
		return place{a.ref, a.restore}
	}
	return place{a.ref, a.reader.Offset()}
}

// follow opens a link's target and remembers where it was followed from.
func (a *App) follow(ref string) tea.Cmd {
	if a.ref != "" {
		a.back = append(a.back, a.here())
		if len(a.back) > maxHistory {
			a.back = append([]place(nil), a.back[len(a.back)-maxHistory:]...)
		}
	}
	return a.openPlace(place{ref: ref})
}

// openPlace opens p.ref the way ":" does, scrolled to p.offset once drawn.
func (a *App) openPlace(p place) tea.Cmd {
	a.focusTree = false
	a.restoreRef, a.restore = p.ref, p.offset
	return a.Open(p.ref)
}

func (a *App) goBack() tea.Cmd {
	if len(a.back) == 0 {
		return nil
	}
	cur, fwd := a.here(), a.fwd
	p := a.back[len(a.back)-1]
	a.back = a.back[:len(a.back)-1]
	cmd := a.openPlace(p)
	a.fwd = append(fwd, cur) // open cleared it
	return cmd
}

func (a *App) goForward() tea.Cmd {
	if len(a.fwd) == 0 {
		return nil
	}
	cur := a.here()
	p := a.fwd[len(a.fwd)-1]
	rest := a.fwd[:len(a.fwd)-1]
	a.back = append(a.back, cur)
	cmd := a.openPlace(p)
	a.fwd = rest
	return cmd
}
