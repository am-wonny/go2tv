package interactive

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alexballas/go2tv/internal/soapcalls"
	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/encoding"
	"github.com/mattn/go-runewidth"
)

// NewScreen .
type NewScreen struct {
	mu         sync.RWMutex
	Current    tcell.Screen
	TV         *soapcalls.TVPayload
	mediaTitle string
	lastAction string
}

var flipflop bool = true

func (p *NewScreen) emitStr(x, y int, style tcell.Style, str string) {
	s := p.Current
	for _, c := range str {
		var comb []rune
		w := runewidth.RuneWidth(c)
		if w == 0 {
			comb = []rune{c}
			c = ' '
			w = 1
		}
		s.SetContent(x, y, c, comb, style)
		x += w
	}
}

// EmitMsg - Display the actions to the interactive terminal.
// Method to implement the screen interface
func (p *NewScreen) EmitMsg(inputtext string) {
	p.updateLastAction(inputtext)
	s := p.Current

	p.mu.RLock()
	mediaTitle := p.mediaTitle
	p.mu.RUnlock()

	titleLen := len("Title: " + mediaTitle)
	w, h := s.Size()
	boldStyle := tcell.StyleDefault.
		Background(tcell.ColorBlack).
		Foreground(tcell.ColorWhite).Bold(true)
	blinkStyle := tcell.StyleDefault.
		Background(tcell.ColorBlack).
		Foreground(tcell.ColorWhite).Blink(true)

	s.Clear()

	p.emitStr(w/2-titleLen/2, h/2-2, tcell.StyleDefault, "Title: "+mediaTitle)
	if inputtext == "Waiting for status..." {
		p.emitStr(w/2-len(inputtext)/2, h/2, blinkStyle, inputtext)
	} else {
		p.emitStr(w/2-len(inputtext)/2, h/2, boldStyle, inputtext)
	}
	p.emitStr(1, 1, tcell.StyleDefault, "Press ESC to stop and exit.")

	isMute := "0"
	var err error

	if p.TV != nil {
		isMute, err = p.TV.GetMuteSoapCall()
	}

	if err != nil || isMute == "0" {
		p.emitStr(w/2-len("")/2, h/2+2, tcell.StyleDefault, "")
	} else {
		p.emitStr(w/2-len("MUTED")/2, h/2+2, blinkStyle, "MUTED")
	}
	p.emitStr(w/2-len(`"p" (Play/Pause)`)/2, h/2+4, tcell.StyleDefault, `"p" (Play/Pause)`)
	p.emitStr(w/2-len(`"m" (Mute/Unmute)`)/2, h/2+6, tcell.StyleDefault, `"m" (Mute/Unmute)`)
	p.emitStr(w/2-len(`"Page Up" "Page Down" (Volume Up/Down)`)/2, h/2+8, tcell.StyleDefault, `"Page Up" "Page Down" (Volume Up/Down)`)
	s.Show()
}

// InterInit - Start the interactive terminal
func (p *NewScreen) InterInit(tv *soapcalls.TVPayload) {
	p.TV = tv

	muteChecker := time.NewTicker(1 * time.Second)

	go func() {
		for range muteChecker.C {
			p.EmitMsg(p.getLastAction())
		}
	}()

	p.mu.Lock()
	p.mediaTitle = tv.MediaURL
	mediaTitlefromURL, err := url.Parse(tv.MediaURL)
	if err == nil {
		p.mediaTitle = strings.TrimLeft(mediaTitlefromURL.Path, "/")
	}
	p.mu.Unlock()

	encoding.Register()
	s := p.Current
	if err := s.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	defStyle := tcell.StyleDefault.
		Background(tcell.ColorBlack).
		Foreground(tcell.ColorWhite)
	s.SetStyle(defStyle)

	p.updateLastAction("Waiting for status...")
	p.EmitMsg(p.getLastAction())

	// Sending the Play1 action sooner may result
	// in a panic error since we need to properly
	// initialize the tcell window.
	if err := tv.SendtoTV("Play1"); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	for {
		switch ev := s.PollEvent().(type) {
		case *tcell.EventResize:
			s.Sync()
			p.EmitMsg(p.getLastAction())
		case *tcell.EventKey:
			p.HandleKeyEvent(ev)
		}
	}
}

// HandleKeyEvent Method to handle all key press events
func (p *NewScreen) HandleKeyEvent(ev *tcell.EventKey) {
	tv := p.TV

	if ev.Key() == tcell.KeyEscape {
		tv.SendtoTV("Stop")
		p.Fini()
	}

	if ev.Key() == tcell.KeyPgUp || ev.Key() == tcell.KeyPgDn {
		currentVolume, err := tv.GetVolumeSoapCall()
		if err != nil {
			return
		}

		setVolume := currentVolume - 1
		if ev.Key() == tcell.KeyPgUp {
			setVolume = currentVolume + 1
		}

		stringVolume := strconv.Itoa(setVolume)

		if err := tv.SetVolumeSoapCall(stringVolume); err != nil {
			return
		}
	}

	switch ev.Rune() {
	case 'p':
		if flipflop {
			flipflop = false
			tv.SendtoTV("Pause")
		} else {
			flipflop = true
			tv.SendtoTV("Play")
		}
	case 'm':
		currentMute, err := tv.GetMuteSoapCall()
		if err != nil {
			break
		}
		switch currentMute {
		case "1":
			if err = tv.SetMuteSoapCall("0"); err == nil {
				p.EmitMsg(p.getLastAction())
			}
		case "0":
			if err = tv.SetMuteSoapCall("1"); err == nil {
				p.EmitMsg(p.getLastAction())
			}
		}
	}
}

// Fini Method to implement the screen interface
func (p *NewScreen) Fini() {
	p.Current.Fini()
	os.Exit(0)
}

// InitTcellNewScreen .
func InitTcellNewScreen() (*NewScreen, error) {
	s, e := tcell.NewScreen()
	if e != nil {
		return nil, errors.New("can't start new interactive screen")
	}

	return &NewScreen{
		Current: s,
	}, nil
}

func (p *NewScreen) getLastAction() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastAction
}

func (p *NewScreen) updateLastAction(s string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastAction = s
}
