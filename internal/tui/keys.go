package tui

const footerHelp = "? help  enter send/edit  alt+enter newline  ctrl+g cancel chat  a chat  / filter  w web  tab screen  l launch  S start  q quit"

func nextScreen(screen appScreen) appScreen {
	switch screen {
	case screenChat:
		return screenDashboard
	case screenDashboard:
		return screenLaunch
	case screenLaunch:
		return screenChains
	case screenChains:
		return screenReceipts
	case screenReceipts:
		return screenChat
	default:
		return screenChat
	}
}
