//go:build android || ios
// +build android ios

package gui

import (
	"os"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/alexballas/go2tv/internal/httphandlers"
	"github.com/alexballas/go2tv/internal/soapcalls"
	"github.com/pkg/errors"
)

// NewScreen .
type NewScreen struct {
	mu                  sync.RWMutex
	Current             fyne.Window
	tvdata              *soapcalls.TVPayload
	Stop                *widget.Button
	MuteUnmute          *widget.Button
	CheckVersion        *widget.Button
	CustomSubsCheck     *widget.Check
	ExternalMediaURL    *widget.Check
	MediaText           *widget.Entry
	SubsText            *widget.Entry
	DeviceList          *widget.List
	httpserver          *httphandlers.HTTPserver
	PlayPause           *widget.Button
	mediafile           fyne.URI
	subsfile            fyne.URI
	selectedDevice      devType
	State               string
	controlURL          string
	eventlURL           string
	renderingControlURL string
	version             string
	mediaFormats        []string
	Medialoop           bool
}

type devType struct {
	name string
	addr string
}

type mainButtonsLayout struct{}

// Start .
func Start(s *NewScreen) {
	w := s.Current

	tabs := container.NewAppTabs(
		container.NewTabItem("Go2TV", container.NewVScroll(container.NewPadded(mainWindow(s)))),
		container.NewTabItem("About", container.NewVScroll(aboutWindow(s))),
	)
	w.SetContent(tabs)
	w.CenterOnScreen()
	w.ShowAndRun()
	os.Exit(0)
}

// EmitMsg Method to implement the screen interface
func (p *NewScreen) EmitMsg(a string) {
	switch a {
	case "Playing":
		setPlayPauseView("Pause", p)
		p.updateScreenState("Playing")
	case "Paused":
		setPlayPauseView("Play", p)
		p.updateScreenState("Paused")
	case "Stopped":
		setPlayPauseView("Play", p)
		p.updateScreenState("Stopped")
		stopAction(p)
	default:
		dialog.ShowInformation("?", "Unknown callback value", p.Current)
	}
}

// Fini Method to implement the screen interface.
// Will only be executed when we receive a callback message,
// not when we explicitly click the Stop button.
func (p *NewScreen) Fini() {
	// Main media loop logic
	if p.Medialoop {
		playAction(p)
	}
}

//InitFyneNewScreen .
func InitFyneNewScreen(v string) *NewScreen {
	go2tv := app.New()
	go2tv.Settings().SetTheme(theme.DarkTheme())

	w := go2tv.NewWindow("Go2TV")

	return &NewScreen{
		Current:      w,
		mediaFormats: []string{".mp4", ".avi", ".mkv", ".mpeg", ".mov", ".webm", ".m4v", ".mpv", ".mp3", ".flac", ".wav"},
		version:      v,
	}
}

func check(win fyne.Window, err error) {
	if err != nil {
		cleanErr := strings.ReplaceAll(err.Error(), ": ", "\n")
		dialog.ShowError(errors.New(cleanErr), win)
	}
}

// updateScreenState updates the screen state based on
// the emitted messages. The State variable is used across
// the GUI interface to control certain flows.
func (p *NewScreen) updateScreenState(a string) {
	p.mu.Lock()
	p.State = a
	p.mu.Unlock()
}

// getScreenState returns the current screen state
func (p *NewScreen) getScreenState() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.State
}

func setPlayPauseView(s string, screen *NewScreen) {
	screen.PlayPause.Enable()
	switch s {
	case "Play":
		screen.PlayPause.Text = "Play"
		screen.PlayPause.Icon = theme.MediaPlayIcon()
		screen.PlayPause.Refresh()
	case "Pause":
		screen.PlayPause.Text = "Pause"
		screen.PlayPause.Icon = theme.MediaPauseIcon()
		screen.PlayPause.Refresh()
	}
}

func setMuteUnmuteView(s string, screen *NewScreen) {
	switch s {
	case "Mute":
		screen.MuteUnmute.Icon = theme.VolumeMuteIcon()
		screen.MuteUnmute.Refresh()
	case "Unmute":
		screen.MuteUnmute.Icon = theme.VolumeUpIcon()
		screen.MuteUnmute.Refresh()
	}
}
