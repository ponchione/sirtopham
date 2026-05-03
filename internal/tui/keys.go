package tui

const footerHelp = "? help  / filter  ctrl+u clear  w web  tab screen  l launch  b preset  B save preset  s save  L load  v preview  S start  F follow  P pause  X cancel  q quit"

func nextScreen(screen appScreen) appScreen {
	switch screen {
	case screenDashboard:
		return screenLaunch
	case screenLaunch:
		return screenChains
	case screenChains:
		return screenReceipts
	case screenReceipts:
		return screenDashboard
	default:
		return screenDashboard
	}
}
