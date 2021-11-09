package httphandlers

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexballas/go2tv/internal/soapcalls"
	"github.com/alexballas/go2tv/internal/utils"
)

// HTTPserver - new http.Server instance.
type HTTPserver struct {
	http *http.Server
	mux  *http.ServeMux
}

// Screen interface.
type Screen interface {
	EmitMsg(string)
	Fini()
}

// Emit .
func Emit(scr Screen, s string) {
	scr.EmitMsg(s)
}

// Close .
func Close(scr Screen) {
	scr.Fini()
}

// ServeFiles - Start HTTP server and serve the files.
func (s *HTTPserver) ServeFiles(serverStarted chan<- struct{}, media, subtitles interface{},
	tvpayload *soapcalls.TVPayload, screen Screen) error {

	mURL, err := url.Parse(tvpayload.MediaURL)
	if err != nil {
		return fmt.Errorf("failed to parse MediaURL: %w", err)
	}

	sURL, err := url.Parse(tvpayload.SubtitlesURL)
	if err != nil {
		return fmt.Errorf("failed to parse SubtitlesURL: %w", err)
	}

	s.mux.HandleFunc(mURL.Path, s.serveMediaHandler(media))
	s.mux.HandleFunc(sURL.Path, s.serveSubtitlesHandler(subtitles))
	s.mux.HandleFunc("/callback", s.callbackHandler(tvpayload, screen))

	ln, err := net.Listen("tcp", s.http.Addr)
	if err != nil {
		return fmt.Errorf("server listen error: %w", err)
	}

	serverStarted <- struct{}{}
	s.http.Serve(ln)

	return nil
}

func (s *HTTPserver) serveMediaHandler(media interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		serveContent(w, req, media, true)
	}
}

func (s *HTTPserver) serveSubtitlesHandler(subs interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		serveContent(w, req, subs, false)
	}
}

func (s *HTTPserver) callbackHandler(tv *soapcalls.TVPayload, screen Screen) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		reqParsed, _ := io.ReadAll(req.Body)
		sidVal, sidExists := req.Header["Sid"]

		if !sidExists {
			http.NotFound(w, req)
			return
		}

		if sidVal[0] == "" {
			http.NotFound(w, req)
			return
		}

		uuid := sidVal[0]
		uuid = strings.TrimLeft(uuid, "[")
		uuid = strings.TrimLeft(uuid, "]")
		uuid = strings.TrimPrefix(uuid, "uuid:")

		// Apparently we should ignore the first message
		// On some media renderers we receive a STOPPED message
		// even before we start streaming.
		seq, err := soapcalls.GetSequence(uuid)
		if err != nil {
			http.NotFound(w, req)
			return
		}

		if seq == 0 {
			soapcalls.IncreaseSequence(uuid)
			fmt.Fprintf(w, "OK\n")
			return
		}

		reqParsedUnescape := html.UnescapeString(string(reqParsed))
		previousstate, newstate, err := soapcalls.EventNotifyParser(reqParsedUnescape)
		if err != nil {
			http.NotFound(w, req)
			return
		}

		if !soapcalls.UpdateMRstate(previousstate, newstate, uuid) {
			http.NotFound(w, req)
			return
		}

		switch newstate {
		case "PLAYING":
			Emit(screen, "Playing")
		case "PAUSED_PLAYBACK":
			Emit(screen, "Paused")
		case "STOPPED":
			Emit(screen, "Stopped")
			tv.UnsubscribeSoapCall(uuid)
			Close(screen)
		}
	}
}

// StopServeFiles .
func (s *HTTPserver) StopServeFiles() {
	s.http.Close()
}

// NewServer - create a new HTTP server.
func NewServer(a string) *HTTPserver {
	mux := http.NewServeMux()
	srv := HTTPserver{
		http: &http.Server{Addr: a, Handler: mux},
		mux:  mux,
	}

	return &srv
}

func serveContent(w http.ResponseWriter, r *http.Request, s interface{}, isMedia bool) {
	respHeader := w.Header()
	if isMedia {
		respHeader["transferMode.dlna.org"] = []string{"Streaming"}
		respHeader["realTimeInfo.dlna.org"] = []string{"DLNA.ORG_TLAG=*"}
	} else {
		respHeader["transferMode.dlna.org"] = []string{"Interactive"}
	}

	switch f := s.(type) {
	case string:
		if r.Header.Get("getcontentFeatures.dlna.org") == "1" {
			contentFeatures, err := utils.BuildContentFeatures(f)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			respHeader["contentFeatures.dlna.org"] = []string{contentFeatures}
		}

		filePath, err := os.Open(f)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer filePath.Close()

		fileStat, err := filePath.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}

		http.ServeContent(w, r, filepath.Base(f), fileStat.ModTime(), filePath)

	case []byte:
		if r.Header.Get("getcontentFeatures.dlna.org") == "1" {
			contentFeatures, _ := utils.BuildContentFeatures("")
			respHeader["contentFeatures.dlna.org"] = []string{contentFeatures}
		}

		bReader := bytes.NewReader(f)

		http.ServeContent(w, r, "", time.Now(), bReader)

	case io.Reader:
		if r.Header.Get("getcontentFeatures.dlna.org") == "1" {
			contentFeatures, _ := utils.BuildContentFeatures("")
			respHeader["contentFeatures.dlna.org"] = []string{contentFeatures}
		}

		// No seek support
		io.Copy(w, f)

	default:
		http.NotFound(w, r)
		return
	}

}
