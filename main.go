package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"openhab_tui/openhab_rest"
)

type model struct {
	widgets  []openhab_rest.Widget
	choices  []string         // items on the to-do list
	cursor   int              // which to-do list item our cursor is pointing at
	selected map[int]struct{} // which to-do items are selected
}

func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Is it a key press?
	case tea.KeyMsg:

		// Cool, what was the actual key pressed?
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q":
			return m, tea.Quit

		// The "up" and "k" keys move the cursor up
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		// The "down" and "j" keys move the cursor down
		case "down", "j":
			if m.cursor < len(m.widgets)-1 {
				m.cursor++
			}

		// The "enter" key and the spacebar (a literal space) toggle
		// the selected state for the item that the cursor is pointing at.
		case "enter", " ":
			if m.widgets[m.cursor].Item.State == "ON" {
				m.widgets[m.cursor].Item.State = "OFF"
				openhab_rest.Set_item(m.widgets[m.cursor].Item.Link, "OFF")
			} else {
				m.widgets[m.cursor].Item.State = "ON"
				openhab_rest.Set_item(m.widgets[m.cursor].Item.Link, "ON")
			}
		}
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func (m model) View() string {
	// The header
	s := ""

	// Iterate over our widgets
	for i, w := range m.widgets {

		// Is the cursor pointing at this choice?
		cursor := " " // no cursor
		if m.cursor == i {
			cursor = ">" // cursor!
		}

		// Is this choice selected?
		checked := " " // not selected
		if w.Item.State == "ON" {
			checked = "x" // selected!
		}

		// Render the row
		s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, w.Label)
	}

	// The footer
	s += "\nPress q to quit.\n"

	// Send the UI for rendering
	return s
}

func initialModel(widgets []openhab_rest.Widget) model {
	var c []string
	for _, w := range widgets {
		c = append(c, w.Label)
	}
	return model{
		// Our shopping list is a grocery list
		widgets: widgets,
		choices: c,

		// A map which indicates which choices are selected. We're using
		// the  map like a mathematical set. The keys refer to the indexes
		// of the `choices` slice, above.
		selected: make(map[int]struct{}),
	}
}

func get_buttons(widgets []openhab_rest.Widget) []openhab_rest.Widget {
	var buttons []openhab_rest.Widget
	for _, w := range widgets {
		if w.Type == "Switch" {
			buttons = append(buttons, w)
		}
		if len(w.Widgets) != 0 {
			buttons = append(buttons, get_buttons(w.Widgets)...)
		}
	}

	return buttons
}

func main() {
	ip := "localhost"
	sitemap_name := "default"
	if len(os.Args[1:]) > 0 {
		ip = os.Args[1]
	}

	if len(os.Args[1:]) > 1 {
		sitemap_name = os.Args[2]
	}

	sitemap := openhab_rest.Get_sitemap(ip, sitemap_name)

	buttons := get_buttons(sitemap.Homepage.Widgets)

	p := tea.NewProgram(initialModel(buttons))
	if err := p.Start(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
