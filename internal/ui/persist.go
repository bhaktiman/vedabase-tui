package ui

// persist writes state.json now, so a closed terminal or a crash keeps the
// reading position and toggles. The file is small and written atomically.
func (a *App) persist() {
	if a.save != nil {
		_ = a.save(a.State())
	}
}

// persistOnOpen saves once per opened item, after its first render, when the
// restored scroll offset is in place.
func (a *App) persistOnOpen() {
	if a.savedRef == a.ref {
		return
	}
	a.savedRef = a.ref
	a.persist()
}
