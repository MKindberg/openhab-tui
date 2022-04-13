package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	lm "github.com/charmbracelet/wish/logging"
	"github.com/gliderlabs/ssh"

	"openhab_tui/openhab_rest"
)

type model struct {
	name     string
	widgets  []openhab_rest.Widget
	cursor   int
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
			new_cursor := m.cursor - 1
			for new_cursor > 0 && len(m.widgets[new_cursor].Actions) == 0 {
				new_cursor--
			}
			if new_cursor > 0 && len(m.widgets[new_cursor].Actions) != 0 {
				m.cursor = new_cursor
			}

		// The "down" and "j" keys move the cursor down
		case "down", "j":
			new_cursor := m.cursor + 1
			for new_cursor < len(m.widgets)-1 && len(m.widgets[new_cursor].Actions) == 0 {
				new_cursor++
			}
			if new_cursor < len(m.widgets) && len(m.widgets[new_cursor].Actions) != 0 {
				m.cursor = new_cursor
			}

		case "right", "l":
			if f, ok := m.widgets[m.cursor].Actions["right"]; ok {
				f(&(m.widgets[m.cursor]))
			}
		case "left", "h":
			if f, ok := m.widgets[m.cursor].Actions["left"]; ok {
				f(&(m.widgets[m.cursor]))
			}
		case "G":
			m.cursor = len(m.widgets) - 1
			for len(m.widgets[m.cursor].Actions) == 0 {
				m.cursor--
			}
		case "g":
			m.cursor = 0
			for len(m.widgets[m.cursor].Actions) == 0 {
				m.cursor++
			}
		// The "enter" key and the spacebar (a literal space) toggle
		// the selected state for the item that the cursor is pointing at.
		case "enter", " ":
			if f, ok := m.widgets[m.cursor].Actions["enter"]; ok {
				f(&(m.widgets[m.cursor]))
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
		depth := " "
		for j := 0; j < w.Depth; j++ {
			depth += "  "
		}

		s += cursor
		s += depth
		s += w.Render(w)
		s += "\n"
	}


	// The footer
	s += "\nPress q to quit.\n"

	// Send the UI for rendering
	return s
}

func initialModel(widgets []openhab_rest.Widget) model {
	cursor := 0
	for widgets[cursor].Type == "Frame" {
		cursor++
	}
	return model{
		// Our shopping list is a grocery list
		widgets: widgets,
		cursor:  cursor,
	}
}

// Only support switches at the moment
func get_supported_widgets(widgets []openhab_rest.Widget, depth int) []openhab_rest.Widget {
	var supported []openhab_rest.Widget
	for _, w := range widgets {
		if w.Visibility == false {
			continue
		}
		w.Actions = make(map[string] func(*openhab_rest.Widget))
		w.Depth = depth
		switch w.Type {
		case "Switch":
			w.Actions["enter"] = func(w *openhab_rest.Widget) {
				if w.Item.State == "ON" {
					w.Item.State = "OFF"
					openhab_rest.Set_item(w.Item.Link, "OFF")
				} else {
					w.Item.State = "ON"
					openhab_rest.Set_item(w.Item.Link, "ON")
				}
				
			}
			w.Render = func(w openhab_rest.Widget) string {
				checked := " " // not selected
				if w.Item.State == "ON" {
					checked = "x" // selected!
				}
				return fmt.Sprintf("%-10s [%s]", w.Label, checked)
			}
			supported = append(supported, w)
		case "Slider":
			w.Actions["left"] = func(w *openhab_rest.Widget) {
				old_val, _ := strconv.Atoi(w.Item.State)
				if old_val > 0 {
					w.Item.State = strconv.Itoa(old_val - 1 - (old_val-1)%5)
					openhab_rest.Set_item(w.Item.Link, w.Item.State)
				}
			}
			w.Actions["right"] = func(w *openhab_rest.Widget) {
				old_val, _ := strconv.Atoi(w.Item.State)
				if old_val < 100 {
					w.Item.State = strconv.Itoa(old_val + 5 - old_val%5)
					openhab_rest.Set_item(w.Item.Link, w.Item.State)
				}
			}
			w.Render = func(w openhab_rest.Widget) string {
				slider := ""
				state, _ := strconv.Atoi(w.Item.State)
				for j := 0; j < state/5; j++ {
					slider += "|"
				}
				for j := 0; j < 20-state/5; j++ {
					slider += " "
				}
				slider += ""
				return fmt.Sprintf("%-10s [%s]", w.Label, slider)
			}
			supported = append(supported, w)
		case "Frame":
			w.Render = func(w openhab_rest.Widget) string {
				return lipgloss.NewStyle().Background(lipgloss.Color("#7D56F4")).Render(w.Label)
			}
			supported = append(supported, w)
			// Flatten frames
			if len(w.Widgets) != 0 {
				supported = append(supported, get_supported_widgets(w.Widgets, depth+1)...)
			}
		default:
			fmt.Println(w.Type + " isn't supported")
		}
	}

	return supported
}

//////////// WISH //////////////
func create_teaHandler(ip string, sitemap_name string) func(ssh.Session) (tea.Model, []tea.ProgramOption) {
	return func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
		sitemap := openhab_rest.Get_sitemap(ip, sitemap_name)
		widgets := get_supported_widgets(sitemap.Homepage.Widgets, 0)
		m := initialModel(widgets)
		return m, []tea.ProgramOption{tea.WithAltScreen()}
	}
}

//////////// MAIN ////////////


func main() {
	var host string
	var ip string
	var sitemap_name string
	var server bool
	var port int

	flag.StringVar(&ip, "ip", "localhost", "IP of the openhab server")
	flag.StringVar(&sitemap_name, "sitemap", "default", "Sitemap to use")
	flag.BoolVar(&server, "server", false, "Start as a server instead of tui")
	flag.StringVar(&host, "host", "localhost", "Ip to host the server on")
	flag.IntVar(&port, "port", 23234, "The port to run the server on")

	flag.Parse()

	if server {
		s, err := wish.NewServer(
			wish.WithAddress(fmt.Sprintf("%s:%d", host, port)),
			wish.WithHostKeyPath(".ssh/term_info_ed25519"),
			wish.WithMiddleware(
				bm.Middleware(create_teaHandler(ip, sitemap_name)),
				lm.Middleware(),
			),
		)

		if err != nil {
			log.Fatalln(err)
		}

		done := make(chan os.Signal, 1)
		signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		log.Printf("Starting SSH server on %s:%d", host, port)
		go func() {
			if err = s.ListenAndServe(); err != nil {
				log.Fatalln(err)
			}
		}()

		<-done
		log.Println("Stopping SSH server")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer func() { cancel() }()
		if err := s.Shutdown(ctx); err != nil {
			log.Fatalln(err)
		}

	} else {
		sitemap := openhab_rest.Get_sitemap(ip, sitemap_name)
		widgets := get_supported_widgets(sitemap.Homepage.Widgets, 0)
		p := tea.NewProgram(initialModel(widgets))
		if err := p.Start(); err != nil {
			fmt.Printf("Alas, there's been an error: %v", err)
			os.Exit(1)
		}
	}
}
