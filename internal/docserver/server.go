package docserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Lanterna-Games/defold-tools/internal/docs"
)

const (
	addressOfThisMachine = "127.0.0.1"
	identityPath         = "/identity"
	nameOfThisProgram    = "defoldpls"
	timeToAnswer         = 10 * time.Second
	timeToBeRecognised   = 500 * time.Millisecond
	pauseBeforeTakingUp  = 3 * time.Second
)

type identity struct {
	Program string `json:"program"`
}

type Server struct {
	Versions      func() []string
	Documentation func(version string) (*docs.Reference, error)
	PortFile      string
	Pause         time.Duration

	loading sync.Mutex
	loaded  map[string]*docs.Reference
}

func (s *Server) documentationOf(version string) (*docs.Reference, error) {
	s.loading.Lock()
	defer s.loading.Unlock()
	if reference, isLoaded := s.loaded[version]; isLoaded {
		return reference, nil
	}
	reference, err := s.Documentation(version)
	if err != nil {
		return nil, err
	}
	if s.loaded == nil {
		s.loaded = map[string]*docs.Reference{}
	}
	s.loaded[version] = reference
	return reference, nil
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.serve)
}

func (s *Server) serve(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		http.Error(response, "this reference is only read", http.StatusMethodNotAllowed)
		return
	}
	versions := s.Versions()
	path := strings.Trim(request.URL.Path, "/")
	switch {
	case request.URL.Path == identityPath:
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(identity{Program: nameOfThisProgram})
		return
	case path == "":
		if len(versions) == 0 {
			http.NotFound(response, request)
			return
		}
		http.Redirect(response, request, "/"+versions[0]+"/", http.StatusFound)
		return
	}
	version, namespace, _ := strings.Cut(path, "/")
	reference, err := s.documentationOf(version)
	if err != nil {
		http.Error(response, err.Error(), http.StatusNotFound)
		return
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	if namespace == "" {
		_ = pages.ExecuteTemplate(response, "version", versionPage{Version: version, Versions: versions, Modules: reference.Modules})
		return
	}
	for _, module := range reference.Modules {
		if module.Namespace == namespace {
			_ = pages.ExecuteTemplate(response, "module", modulePage{
				Version: version, Versions: versions, Module: module, OnTheWeb: referenceOnTheWeb + namespace + "-lua",
			})
			return
		}
	}
	http.Error(response, fmt.Sprintf("Defold %s documents no %q", version, namespace), http.StatusNotFound)
}

func (s *Server) Start(ctx context.Context) (address string, err error) {
	if remembered, isRemembered := s.rememberedPort(); isRemembered {
		if listener, err := listenOn(remembered); err == nil {
			go s.serveUntilDone(ctx, listener)
			return addressOf(remembered), nil
		}
		if isServedByThisProgram(remembered) {
			go s.takeUpWhenLeftAlone(ctx, remembered)
			return addressOf(remembered), nil
		}
	}
	listener, err := listenOn(0)
	if err != nil {
		return "", fmt.Errorf("opening a port for the offline reference: %w", err)
	}
	assigned, isTCP := listener.Addr().(*net.TCPAddr)
	if !isTCP {
		_ = listener.Close()
		return "", errors.New("the system handed out an address that is not a port")
	}
	port := assigned.Port
	s.remember(port)
	go s.serveUntilDone(ctx, listener)
	return addressOf(port), nil
}

func listenOn(port int) (net.Listener, error) {
	var listening net.ListenConfig
	return listening.Listen(context.Background(), "tcp", net.JoinHostPort(addressOfThisMachine, strconv.Itoa(port)))
}

func addressOf(port int) string {
	return "http://" + net.JoinHostPort(addressOfThisMachine, strconv.Itoa(port))
}

func (s *Server) serveUntilDone(ctx context.Context, listener net.Listener) {
	server := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: timeToAnswer}
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return
	}
}

func (s *Server) takeUpWhenLeftAlone(ctx context.Context, port int) {
	every := s.Pause
	if every == 0 {
		every = pauseBeforeTakingUp
	}
	pause := time.NewTicker(every)
	defer pause.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-pause.C:
			if listener, err := listenOn(port); err == nil {
				s.serveUntilDone(ctx, listener)
				return
			}
		}
	}
}

func isServedByThisProgram(port int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeToBeRecognised)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, addressOf(port)+identityPath, http.NoBody)
	if err != nil {
		return false
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	var answered identity
	return json.NewDecoder(response.Body).Decode(&answered) == nil && answered.Program == nameOfThisProgram
}

func (s *Server) rememberedPort() (port int, isRemembered bool) {
	if s.PortFile == "" {
		return 0, false
	}
	written, err := os.ReadFile(s.PortFile)
	if err != nil {
		return 0, false
	}
	port, err = strconv.Atoi(strings.TrimSpace(string(written)))
	return port, err == nil && port > 0 && port < 65536
}

func (s *Server) remember(port int) {
	if s.PortFile == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.PortFile), 0o750); err != nil {
		return
	}
	_ = os.WriteFile(s.PortFile, []byte(strconv.Itoa(port)+"\n"), 0o600)
}
