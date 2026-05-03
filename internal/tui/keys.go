package tui

func (m Model) footerHelp() string {
	switch m.screen {
	case screenChat:
		if m.chatEdit {
			return "? help  enter send  alt+enter/ctrl+j newline  esc stop edit  ctrl+u clear"
		}
		if m.chatRunning {
			return "? help  ctrl+g cancel  tab screen  d dashboard  q quit"
		}
		return "? help  enter/i edit  N new chat  tab screen  d dashboard  q quit"
	case screenDashboard:
		return "? help  enter chains  r refresh  w web  tab screen  q quit"
	case screenLaunch:
		return "? help  i edit  b preset  m mode  n add role  v preview  S start  q quit"
	case screenChains:
		return "? help  enter receipts  F follow  P pause  X cancel  / filter  tab screen  q quit"
	case screenReceipts:
		return "? help  o pager  E editor  esc chains  / filter  tab screen  q quit"
	case screenHelp:
		return "? close help  tab previous  q quit"
	default:
		return "? help  tab screen  q quit"
	}
}

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
