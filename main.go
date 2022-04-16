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

//////////////////// OPTIONS ////////////////////
type options struct {
	host         string
	ip           string
	sitemap_name string
	server       bool
	port         int
}

func (o *options) init() {
	flag.StringVar(&o.ip, "ip", "localhost", "IP of the openhab server")
	flag.StringVar(&o.sitemap_name, "sitemap", "default", "Sitemap to use")
	flag.BoolVar(&o.server, "server", false, "Start as a server instead of tui")
	flag.StringVar(&o.host, "host", "localhost", "Ip to host the server on")
	flag.IntVar(&o.port, "port", 23234, "The port to run the server on")

	flag.Parse()
}

var opt options

//////////////////// ELEMENT ////////////////////

type Element interface {
	toString() string
	left()
	right()
	enter()
	interactable() bool
}

type Switch struct {
	label string
	depth int
	item  openhab_rest.Item
}

func (s Switch) toString() string {
	status := " "
	if s.item.State == "ON" {
		status = "X"
	}
	offset := ""
	for i := 0; i < s.depth; i++ {
		offset += "  "
	}
	return fmt.Sprintf("%s%-10s [%s]", offset, s.label, status)
}
func (s Switch) left() {
}
func (s Switch) right() {
}
func (s Switch) enter() {
	if s.item.State == "ON" {
		s.item.State = "OFF"
		openhab_rest.Set_item(s.item.Link, "OFF")
	} else {
		s.item.State = "ON"
		openhab_rest.Set_item(s.item.Link, "ON")
	}
}
func (s Switch) interactable() bool {
	return true
}

type Slider struct {
	label string
	depth int
	item  openhab_rest.Item
}

func (s Slider) toString() string {
	slider := ""
	state, _ := strconv.Atoi(s.item.State)
	for j := 0; j < state/5; j++ {
		slider += "|"
	}
	for j := 0; j < 20-state/5; j++ {
		slider += " "
	}

	offset := ""
	for i := 0; i < s.depth; i++ {
		offset += "  "
	}
	return fmt.Sprintf("%s%-10s [%s]", offset, s.label, slider)
}
func (s Slider) left() {
	old_val, _ := strconv.Atoi(s.item.State)
	if old_val > 0 {
		s.item.State = strconv.Itoa(old_val - 1 - (old_val-1)%5)
		openhab_rest.Set_item(s.item.Link, s.item.State)
	}
}
func (s Slider) right() {
	old_val, _ := strconv.Atoi(s.item.State)
	if old_val < 100 {
		s.item.State = strconv.Itoa(old_val + 5 - old_val%5)
		openhab_rest.Set_item(s.item.Link, s.item.State)
	}
}
func (s Slider) enter() {
}
func (s Slider) interactable() bool {
	return true
}

type Frame struct {
	label string
	depth int
	item  openhab_rest.Item
}

func (s Frame) toString() string {

	offset := ""
	for i := 0; i < s.depth; i++ {
		offset += "  "
	}
	return fmt.Sprintf("%s%s", offset, lipgloss.NewStyle().Background(lipgloss.Color("#7D56F4")).Render(s.label))
}
func (s Frame) left() {
}
func (s Frame) right() {
}
func (s Frame) enter() {
}
func (s Frame) interactable() bool {
	return false
}

/////////////////////////////////////////////////
type model struct {
	elem   []Element
	cursor int
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
			for new_cursor > 0 && !m.elem[new_cursor].interactable() {
				new_cursor--
			}
			if new_cursor > 0 && m.elem[new_cursor].interactable() {
				m.cursor = new_cursor
			}

		// The "down" and "j" keys move the cursor down
		case "down", "j":
			new_cursor := m.cursor + 1
			for new_cursor < len(m.elem)-1 && !m.elem[new_cursor].interactable() {
				new_cursor++
			}
			if new_cursor < len(m.elem) && m.elem[new_cursor].interactable() {
				m.cursor = new_cursor
			}

		case "right", "l":
			m.elem[m.cursor].right()
		case "left", "h":
			m.elem[m.cursor].left()
		case "G":
			m.cursor = len(m.elem) - 1
			for !m.elem[m.cursor].interactable() {
				m.cursor--
			}
		case "g":
			m.cursor = 0
			for !m.elem[m.cursor].interactable() {
				m.cursor++
			}
		// The "enter" key and the spacebar (a literal space) toggle
		// the selected state for the item that the cursor is pointing at.
		case "enter", " ":
			m.elem[m.cursor].enter()
		}
	}
	sitemap := openhab_rest.Get_sitemap(opt.ip, opt.sitemap_name)
	elements := get_supported_widgets(sitemap.Homepage.Widgets, 0)
	m.elem = elements

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func (m model) View() string {
	// The header
	s := ""

	// Iterate over our widgets
	for i, w := range m.elem {

		// Is the cursor pointing at this choice?
		cursor := " " // no cursor
		if m.cursor == i {
			cursor = ">" // cursor!
		}

		s += cursor
		s += w.toString()
		s += "\n"
	}

	// The footer
	s += "\nPress q to quit.\n"

	// Send the UI for rendering
	return s
}

func initialModel(elements []Element) model {
	cursor := 0
	for !elements[cursor].interactable() {
		cursor++
	}
	return model{
		// Our shopping list is a grocery list
		elem:   elements,
		cursor: cursor,
	}
}

// Only support switches at the moment
func get_supported_widgets(widgets []openhab_rest.Widget, depth int) []Element {
	var elements []Element
	for _, w := range widgets {
		if w.Visibility == false {
			continue
		}
		switch w.Type {
		case "Switch":
			// Sometimes w.Item.State is a number,
			// we should always be able to rely on w.State though
			if w.State == "ON" {
				w.Item.State = "ON"
			} else if w.State == "OFF" {
				w.Item.State = "OFF"
			}
			elements = append(elements, Switch{w.Label, depth, w.Item})
		case "Slider":
			elements = append(elements, Slider{w.Label, depth, w.Item})
		case "Frame":
			elements = append(elements, Frame{w.Label, depth, w.Item})
			// Flatten frames
			if len(w.Widgets) != 0 {
				e := get_supported_widgets(w.Widgets, depth+1)
				elements = append(elements, e...)
			}
		default:
			fmt.Println(w.Type + " isn't supported")
		}
	}

	return elements
}

//////////// WISH //////////////
func create_teaHandler(ip string, sitemap_name string) func(ssh.Session) (tea.Model, []tea.ProgramOption) {
	return func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
		sitemap := openhab_rest.Get_sitemap(ip, sitemap_name)
		elements := get_supported_widgets(sitemap.Homepage.Widgets, 0)
		m := initialModel(elements)
		return m, []tea.ProgramOption{tea.WithAltScreen()}
	}
}

//////////// MAIN ////////////

func main() {

	opt.init()

	if opt.server {
		s, err := wish.NewServer(
			wish.WithAddress(fmt.Sprintf("%s:%d", opt.host, opt.port)),
			wish.WithHostKeyPath(".ssh/term_info_ed25519"),
			wish.WithMiddleware(
				bm.Middleware(create_teaHandler(opt.ip, opt.sitemap_name)),
				lm.Middleware(),
			),
		)

		if err != nil {
			log.Fatalln(err)
		}

		done := make(chan os.Signal, 1)
		signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		log.Printf("Starting SSH server on %s:%d", opt.host, opt.port)
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
		sitemap := openhab_rest.Get_sitemap(opt.ip, opt.sitemap_name)
		elements := get_supported_widgets(sitemap.Homepage.Widgets, 0)
		p := tea.NewProgram(initialModel(elements))
		if err := p.Start(); err != nil {
			fmt.Printf("Alas, there's been an error: %v", err)
			os.Exit(1)
		}
	}
}
