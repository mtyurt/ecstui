package utils

import "github.com/charmbracelet/lipgloss"

var (
	runningStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#80C904")) // Soft Green
	activatingStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBF00")) // Amber
	deactivatingStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBF00")) // Amber
	pendingStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))     // Purple
	stoppingStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))     // Purple
	provisioningStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ADD8E6")) // Light Blue
	deprovisioningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ADD8E6")) // Light Blue
	stoppedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF007A")) // Light Salmon (Soft Red)
)

// MapStatusToLabel maps the input status string to a label with the corresponding style.
func MapTaskStatusToLabel(status string) string {
	var style lipgloss.Style
	var arrow string

	switch status {
	case "RUNNING":
		style = runningStyle
		arrow = "↑"
	case "ACTIVATING":
		style = activatingStyle
		arrow = "↑"
	case "DEACTIVATING":
		style = deactivatingStyle
		arrow = "↓"
	case "PENDING":
		style = pendingStyle
		arrow = "↑"
	case "STOPPING":
		style = stoppingStyle
		arrow = "↓"
	case "PROVISIONING":
		style = provisioningStyle
		arrow = "↑"
	case "DEPROVISIONING":
		style = deprovisioningStyle
		arrow = "↓"
	case "STOPPED":
		style = stoppedStyle
		arrow = "↓"
	default:
		style = lipgloss.NewStyle() // Default style (no background)
	}

	return style.Render(arrow + status)
}
