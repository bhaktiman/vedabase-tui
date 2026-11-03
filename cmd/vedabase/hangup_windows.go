//go:build windows

package main

import tea "charm.land/bubbletea/v2"

// quitOnHangup does nothing: Windows has no SIGHUP.
func quitOnHangup(*tea.Program) {}
