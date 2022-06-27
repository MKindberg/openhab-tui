package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
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
type Base struct {
	label       string
	depth       int
	item        openhab_rest.Item
	parentLabel string
}

type Element interface {
	toString() string
	left()
	right()
	enter()
	interactable() bool
	getBase() Base
}

type Switch struct {
	b Base
}

func (s Switch) toString() string {
	status := " "
	if s.b.item.State == "ON" {
		status = "X"
	}
	offset := ""
	for i := 0; i < s.b.depth; i++ {
		offset += "  "
	}
	return fmt.Sprintf("%s%-10s [%s]", offset, s.b.label, status)
}
func (s Switch) left() {
}
func (s Switch) right() {
}
func (s Switch) enter() {
	if s.b.item.State == "ON" {
		s.b.item.State = "OFF"
		openhab_rest.Set_item(s.b.item.Link, "OFF")
	} else {
		s.b.item.State = "ON"
		openhab_rest.Set_item(s.b.item.Link, "ON")
	}
	// Sleep 1ms so openhab has time to set state before we fetch
	time.Sleep(time.Millisecond)
}
func (s Switch) interactable() bool {
	return true
}
func (s Switch) getBase() Base {
	return s.b
}

type Slider struct {
	b Base
}

func (s Slider) toString() string {
	slider := ""
	state, _ := strconv.Atoi(s.b.item.State)
	for j := 0; j < state/5; j++ {
		slider += "|"
	}
	for j := 0; j < 20-state/5; j++ {
		slider += " "
	}

	offset := ""
	for i := 0; i < s.b.depth; i++ {
		offset += "  "
	}
	return fmt.Sprintf("%s%-10s [%s]", offset, s.b.label, slider)
}
func (s Slider) left() {
	old_val, _ := strconv.Atoi(s.b.item.State)
	if old_val > 0 {
		s.b.item.State = strconv.Itoa(old_val - 1 - (old_val-1)%5)
		openhab_rest.Set_item(s.b.item.Link, s.b.item.State)
	}
	// Sleep 1ms so openhab has time to set state before we fetch
	time.Sleep(time.Millisecond)
}
func (s Slider) right() {
	old_val, _ := strconv.Atoi(s.b.item.State)
	if old_val < 100 {
		s.b.item.State = strconv.Itoa(old_val + 5 - old_val%5)
		openhab_rest.Set_item(s.b.item.Link, s.b.item.State)
	}
	// Sleep 1ms so openhab has time to set state before we fetch
	time.Sleep(time.Millisecond)
}
func (s Slider) enter() {
}
func (s Slider) interactable() bool {
	return true
}
func (s Slider) getBase() Base {
	return s.b
}

type Frame struct {
	b Base
}

func (s Frame) toString() string {

	offset := ""
	for i := 0; i < s.b.depth; i++ {
		offset += "  "
	}
	return fmt.Sprintf("%s%s", offset, lipgloss.NewStyle().Background(lipgloss.Color("#7D56F4")).Render(s.b.label))
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
func (s Frame) getBase() Base {
	return s.b
}

/////////////////////////////////////////////////
type model struct {
	search textinput.Model
	elem   []Element
	cursor int
}

func (m model) Init() tea.Cmd {
	return nil
}

func nextElement(current int, direction int, elements []Element) int {
	cursor := current + direction
	for 0 < cursor && cursor < len(elements)-1 && !elements[cursor].interactable() {
		cursor += direction
	}
	if 0 <= cursor && cursor < len(elements) && elements[cursor].interactable() {
		return cursor
	}
	return current
}

func search(haystacks []string, needles []string) bool {
	for _, n := range needles {
		found := false
		for _, h := range haystacks {
			if strings.Contains(h, n) {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:

		switch msg.String() {

		case "ctrl+c", "esc":
			return m, tea.Quit

		case "up":
			m.cursor = nextElement(m.cursor, -1, m.elem)

		case "down":
			m.cursor = nextElement(m.cursor, 1, m.elem)

		case "right":
			m.elem[m.cursor].right()
		case "left":
			m.elem[m.cursor].left()
		case "end":
			m.cursor = len(m.elem) - 1
			if !m.elem[m.cursor].interactable() {
				m.cursor = nextElement(m.cursor, -1, m.elem)
			}
		case "home":
			m.cursor = 0
			if !m.elem[m.cursor].interactable() {
				m.cursor = nextElement(m.cursor, 1, m.elem)
			}
		case "enter":
			print(m.cursor)
			m.elem[m.cursor].enter()
		}
	}
	m.search, _ = m.search.Update(msg)
	sitemap := openhab_rest.Get_sitemap(opt.ip, opt.sitemap_name)
	elements := get_supported_widgets(sitemap.Homepage.Widgets, 0, "")
	m.elem = []Element{}
	for _, e := range elements {
		terms := strings.Split(strings.ToLower(m.search.Value()), " ")
		l := strings.ToLower(e.getBase().label)
		pl := strings.ToLower(e.getBase().parentLabel)
		if search([]string{l, pl}, terms) {
			m.elem = append(m.elem, e)
		}
	}
	if len(m.elem) > 0 && m.cursor > len(m.elem)-1 {
		m.cursor = len(m.elem) - 1
		if !m.elem[m.cursor].interactable() {
			m.cursor = nextElement(m.cursor, -1, m.elem)
		}
	}

	return m, nil
}

func (m model) View() string {
	s := ""

	s += m.search.View() + "\n\n"
	for i, w := range m.elem {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		s += cursor
		s += w.toString()
		s += "\n"
	}

	s += "\nPress Ctrl+c or Esc to quit.\n"

	return s
}

func initialModel(elements []Element) model {
	cursor := 0
	for !elements[cursor].interactable() {
		cursor++
	}
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20
	return model{
		search: ti,
		elem:   elements,
		cursor: cursor,
	}
}

func get_supported_widgets(widgets []openhab_rest.Widget, depth int, parent string) []Element {
	var elements []Element
	for _, w := range widgets {
		if w.Visibility == false {
			continue
		}
		switch w.Type {
		case "Switch":
			if w.State == "ON" {
				w.Item.State = "ON"
			} else if w.State == "OFF" {
				w.Item.State = "OFF"
			}
			elements = append(elements, Switch{Base{w.Label, depth, w.Item, parent}})
		case "Slider":
			elements = append(elements, Slider{Base{w.Label, depth, w.Item, parent}})
		case "Frame":
			elements = append(elements, Frame{Base{w.Label, depth, w.Item, parent}})
			// Flatten frames
			if len(w.Widgets) != 0 {
				e := get_supported_widgets(w.Widgets, depth+1, w.Label)
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
		elements := get_supported_widgets(sitemap.Homepage.Widgets, 0, "")
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
		elements := get_supported_widgets(sitemap.Homepage.Widgets, 0, "")
		p := tea.NewProgram(initialModel(elements))
		if err := p.Start(); err != nil {
			fmt.Printf("Alas, there's been an error: %v", err)
			os.Exit(1)
		}
	}
}
