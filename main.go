package main

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SetupMessage struct{ projectPath string }
type ExtraDepsMessage struct {
	dependencies []Dependency
}
type InstallAllMsg struct{}
type OnInstalledMsg struct{ index int }
type AuditMsg struct{ audit npmAuditJSON }
type ErrorMsg struct{ error error }
type ExecError struct {
	stderr string
	stdout string
	error  error
}

type PageNumber int

const (
	Page1View PageNumber = iota
	Page2View
	Page3View
	Page4View
	Page5View
)

type Dependency struct {
	name          string
	selected      bool
	devDependency bool
	installing    bool
	installed     bool
}

type Model struct {
	view         PageNumber
	dependencies []Dependency
	audit        npmAuditJSON
	projectPath  string
	installCount int
	cursor       int
	error        string
	textinput    textinput.Model
	spinner      spinner.Model
	stopwatch    stopwatch.Model
}

func (m Model) Init() tea.Cmd {
	return tea.Cmd(m.spinner.Tick)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc || msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyCtrlD {
			return m, tea.Quit
		}

		if m.view == Page1View {
			if msg.Type == tea.KeyEnter {
				return m, func() tea.Msg { return setupProject(&m) }
			}

			var cmd tea.Cmd
			m.textinput, cmd = m.textinput.Update(msg)

			return m, cmd
		}

		if m.view == Page2View {
			if msg.Type == tea.KeySpace || msg.Type == tea.KeyLeft || msg.Type == tea.KeyRight {
				if m.dependencies[m.cursor].selected {
					m.dependencies[m.cursor].selected = false
				} else {
					m.dependencies[m.cursor].selected = true
				}
				return m, nil
			}

			if msg.Type == tea.KeyUp {
				if m.cursor > 0 {
					m.cursor--
				}
				return m, nil
			}

			if msg.Type == tea.KeyDown {
				if m.cursor < len(m.dependencies)-1 {
					m.cursor++
				}
				return m, nil
			}

			if msg.Type == tea.KeyEnter {
				m.view = Page3View
				return m, func() tea.Msg { return extraDependencies(&m) }
			}
		}

	case SetupMessage:
		m.projectPath = msg.projectPath
		m.view = Page2View
		return m, nil

	case ExtraDepsMessage:
		m.dependencies = append(m.dependencies, msg.dependencies...)
		cmds := []tea.Cmd{
			m.stopwatch.Start(),
			func() tea.Msg { return InstallAllMsg{} },
		}

		return m, tea.Sequence(cmds...)

	case InstallAllMsg:
		for i := range m.dependencies {
			if m.dependencies[i].selected && !m.dependencies[i].installed && !m.dependencies[i].installing {
				m.dependencies[i].installing = true

				return m, func() tea.Msg { return installDependency(&m, i) }
			}
		}

		m.view = Page4View
		return m, func() tea.Msg { return runAudit(&m) }

	case OnInstalledMsg:
		m.dependencies[msg.index].installing = false
		m.dependencies[msg.index].installed = true
		m.installCount++

		return m, func() tea.Msg { return InstallAllMsg{} }

	case AuditMsg:
		m.audit = msg.audit
		m.view = Page5View
		return m, tea.Sequence(nil, m.stopwatch.Stop(), tea.Quit)

	case ErrorMsg:
		m.error = msg.error.Error()
		return m, tea.Quit

	default:
		var spinnerCmd tea.Cmd
		var stopwatchCmd tea.Cmd

		m.spinner, spinnerCmd = m.spinner.Update(msg)
		m.stopwatch, stopwatchCmd = m.stopwatch.Update(msg)

		return m, tea.Batch(spinnerCmd, stopwatchCmd)
	}

	return m, nil
}

func (m Model) View() string {
	view := ""

	if m.error != "" {
		view = fmt.Sprintf(" %v %v\n%v\n", elm.cross, style.error.Render("There was an error"), m.error)
		return view
	}

	if m.view == Page1View {
		view = fmt.Sprintf(" %v Enter a name for your project: %v", style.textBlue.Render("?"), m.textinput.View())
	}

	if m.view == Page2View {
		view = fmt.Sprintf(" %v Project name: %v\n", elm.check, m.textinput.Value())
		view += fmt.Sprintf(" %v Select dependencies to install:\n\n", elm.question)

		for i := 0; i < len(m.dependencies); i++ {
			checked := " "

			if m.dependencies[i].selected {
				checked = elm.check
			}

			if m.cursor == i {
				view += fmt.Sprintf(" ❯ ❪%v❫ %v\n", checked, m.dependencies[i].name)
			} else if m.dependencies[i].selected {
				view += fmt.Sprintf("   ❪%v❫ %v\n", checked, m.dependencies[i].name)
			} else {
				view += fmt.Sprintf("   ❪%v❫ %v\n", checked, style.textGray.Render(m.dependencies[i].name))
			}
		}
	}

	if m.view == Page3View {
		view = fmt.Sprintf(" %v Project name: %v\n", elm.check, m.textinput.Value())
		view += fmt.Sprintf(" %v Installing dependencies... \n", elm.check)

		for i, dep := range m.dependencies {
			if m.dependencies[i].installing {
				view += fmt.Sprintf(" %v Installing: %v (%v)\n", m.spinner.View(), dep.name, m.stopwatch.View())
			}
		}
	}

	if m.view == Page4View {
		view = fmt.Sprintf(" %v Project name: %v\n", elm.check, m.textinput.Value())
		view += fmt.Sprintf(" %v Installed %v dependencies.\n", elm.check, m.installCount)
		view += fmt.Sprintf(" %v Running npm audit.\n", m.spinner.View())
	}

	if m.view == Page5View {
		statusText := ""
		severityStatus := severityStatus(&m.audit)

		if m.audit.Metadata.Vulnerabilities.Total > 0 {
			statusText = fmt.Sprintf("Found %v vulenrabilities. Run \"npm audit\" to fix.", m.audit.Metadata.Vulnerabilities.Total)
		} else {
			statusText = "Npm audit found no vulenrabilities"
		}

		view = fmt.Sprintf(" %v Project name: %v\n", elm.check, m.textinput.Value())
		view += fmt.Sprintf(" %v Installed %v dependencies.\n", elm.check, m.installCount)
		view += fmt.Sprintf(" %v %v%v\n", elm.check, severityStatus, statusText)
		view += fmt.Sprintf(" %v %v Project setup complete in %v\n", elm.check, style.success.Render("Success"), m.stopwatch.Elapsed())
	}

	if m.view != Page5View {
		view += fmt.Sprintf("\n %v \n", style.textGray.Render("? Press ESC or Ctrl+C to exit."))
	}

	return view
}

func initialModel() Model {
	ti := textinput.New()
	ti.Placeholder = "project-name"
	ti.Focus()
	ti.Prompt = ""

	loader := spinner.New()
	loader.Spinner = spinner.MiniDot
	loader.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#0ea5e9"))

	watch := stopwatch.NewWithInterval(time.Millisecond)

	return Model{
		spinner:   loader,
		stopwatch: watch,
		textinput: ti,
		dependencies: []Dependency{
			{name: "typescript", selected: true, devDependency: true},
			{name: "react", selected: true, devDependency: false},
			{name: "kysely", selected: true, devDependency: false},
			{name: "esbuild", selected: true, devDependency: true},
			{name: "tailwindcss", selected: true, devDependency: true},
			{name: "nodemon", selected: true, devDependency: true},
			{name: "dotenv", selected: true, devDependency: true},
		},
	}
}

func main() {
	model := initialModel()
	program := tea.NewProgram(model)

	_, err := program.Run()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// if data, ok := data.()
}
