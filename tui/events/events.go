package events

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

var (
	titleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	infoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return titleStyle.Copy().BorderStyle(b)
	}()

	timestampStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "236", Dark: "248"})

	filterPrompt = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#ECFD65"})

	filterCursor = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"})
)

type Model struct {
	eventsView    viewport.Model
	title         string
	events        []*ecs.ServiceEvent
	filterInput   textinput.Model
	filterEnabled bool
}

func New(title string, width, height int, events []*ecs.ServiceEvent) Model {
	view := viewport.New(width, height)
	view.SetYOffset(1)
	filterInput := textinput.New()
	filterInput.Prompt = "Filter: "
	filterInput.Width = width - 10 - len(filterInput.Prompt)
	filterInput.Focus()

	m := Model{
		eventsView:  view,
		title:       title,
		events:      events,
		filterInput: filterInput,
	}

	m.updateContent()
	return m
}

func wrapEventMessage(message string, width, padding int) string {
	wrapPrefix := strings.Repeat(" ", padding)
	wrapped := wordwrap.String(message, width)
	lines := strings.Split(wrapped, "\n")
	return strings.Join(lines, "\n"+wrapPrefix)
}

func (m *Model) updateContent() {
	width := m.eventsView.Width
	var summary []string
	for _, event := range m.events {
		if m.filterEnabled && !strings.Contains(strings.ToLower(*event.Message), strings.ToLower(m.filterInput.Value())) {
			continue
		}

		msg := wrapEventMessage(*event.Message, width-25, 24)
		timestamp := timestampStyle.Render(event.CreatedAt.Format("2006-01-02 15:04:05.000"))
		if m.filterEnabled {
			msg = highlightOccurencesCaseInsensitive(msg, m.filterInput.Value())
		}
		summary = append(summary, fmt.Sprintf("%s %s", timestamp, msg))
	}
	content := strings.Join(summary, "\n")
	m.eventsView.SetContent(content)
}

func highlightOccurencesCaseInsensitive(a, b string) string {
	if b == "" {
		return a
	}

	// Convert both strings to lower case for case-insensitive comparison
	lowerA := strings.ToLower(a)
	lowerB := strings.ToLower(b)

	// Find all occurrences of b in a
	var lastIndex int
	var result strings.Builder
	for {
		index := strings.Index(lowerA[lastIndex:], lowerB)
		if index == -1 {
			break
		}
		// Append the original text plus the highlighted occurrence
		result.WriteString(a[lastIndex : lastIndex+index])
		result.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(a[lastIndex+index : lastIndex+index+len(b)]))
		lastIndex += index + len(b)
	}
	// Append any remaining text after the last occurrence
	result.WriteString(a[lastIndex:])

	return result.String()
}

func (m Model) SetSize(width, height int) {
	m.eventsView.Width = width
	m.eventsView.Height = height
}
func (m Model) Focused() bool {
	return !m.filterEnabled
}
func (m Model) Init() tea.Cmd {
	return nil
}

func (m *Model) clearFilter() {
	m.filterInput.SetValue("")
	m.updateContent()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	cmds := []tea.Cmd{}

	newFilter := false
	if msg, ok := msg.(tea.KeyMsg); ok {
		if !m.filterEnabled {
			switch msg.String() {
			case "/":
				m.filterEnabled = true
				m.filterInput.Focus()

				newFilter = true
			}
		} else {
			switch msg.String() {
			case "esc":
				m.filterEnabled = false
				m.clearFilter()
			}
		}
	}

	if m.filterEnabled && !newFilter {
		newFilterInputModel, inputCmd := m.filterInput.Update(msg)
		m.filterInput = newFilterInputModel
		cmds = append(cmds, inputCmd)
		m.updateContent()
	} else {
		m.eventsView, cmd = m.eventsView.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) headerView() string {
	if m.filterEnabled {
		view := titleStyle.Render(m.filterInput.View())
		line := strings.Repeat("─", max(0, m.eventsView.Width-lipgloss.Width(view)))
		return lipgloss.JoinHorizontal(lipgloss.Center, view, line)
	}
	title := titleStyle.Render(m.title)
	line := strings.Repeat("─", max(0, m.eventsView.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m Model) footerView() string {
	info := infoStyle.Render(fmt.Sprintf("%3.f%%", m.eventsView.ScrollPercent()*100))
	line := strings.Repeat("─", max(0, m.eventsView.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func (m Model) View() string {
	return lipgloss.NewStyle().Margin(5, 1).Render(
		fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.eventsView.View(), m.footerView()))
}
