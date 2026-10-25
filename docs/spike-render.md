# Spike: long-document rendering

Fixture: `660219bg-new-york` purport, 114,801 characters (117,938 bytes).
Machine: AMD Ryzen AI Max+ 395, Go 1.26.6, glamour v2.0.1, bubbles v2.2.1.

| Measurement | Result |
|---|---|
| glamour render, width 80 (bench, 5x) | 130 ms/op (single run: 135 ms, 959 KB output) |
| glamour render, width 200 (bench, 5x) | 185 ms/op |
| `viewport.SetContent` (80-col output, 100x40) | 4.4 ms |
| 500 x `ScrollDown(1)` + `View()` | 585 ms total (about 1.2 ms each) |

## Decision

Render time is well under 1 s, so Task 11 renders the whole document in a
background `tea.Cmd`, showing the loader only if it takes over 100 ms.
Rendering is never on the `Update` path. Scrolling cost is negligible.
Re-render on resize also goes through a background `tea.Cmd`.
