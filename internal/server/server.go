package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

	"web-server/internal/config"
	"web-server/internal/proxy"
	"web-server/internal/static"
)

// Server represents the main web server
type Server struct {
	config  *config.Config
	servers []*http.Server
}

// NewServer creates a new server instance
func NewServer(config *config.Config) *Server {
	return &Server{
		config:  config,
		servers: make([]*http.Server, 0),
	}
}

// Start starts the server
func (s *Server) Start() error {
	if err := s.config.ValidateConfig(); err != nil {
		return fmt.Errorf("configuration validation failed: %v", err)
	}

	// Print the configuration for debugging
	s.config.PrintConfig()

	// Create HTTP servers for each server block
	for _, serverConfig := range s.config.Servers {
		mux := http.NewServeMux()

		// Sort locations by path length (longest first) to ensure proper matching
		locations := make([]config.LocationConfig, len(serverConfig.Locations))
		copy(locations, serverConfig.Locations)
		sort.Slice(locations, func(i, j int) bool {
			return len(locations[i].Path) > len(locations[j].Path)
		})

		// Register handlers for each location
		for _, location := range locations {
			var handler http.Handler

			if location.ProxyPass != "" {
				// Check if proxy_pass refers to an upstream
				var upstream *config.UpstreamConfig
				if strings.HasPrefix(location.ProxyPass, "http://") || strings.HasPrefix(location.ProxyPass, "https://") {
					// Direct proxy - create a single server upstream
					upstream = &config.UpstreamConfig{
						Name:    "direct",
						Servers: []string{location.ProxyPass},
					}
				} else {
					// Upstream reference
					upstream = s.config.GetUpstreamByName(location.ProxyPass)
					if upstream == nil {
						return fmt.Errorf("upstream '%s' not found for location '%s'", location.ProxyPass, location.Path)
					}
				}

				proxyHandler := proxy.NewProxyHandler(upstream, &location)
				handler = http.HandlerFunc(proxyHandler.Handle)
			} else if location.Root != "" {
				// Static file serving
				staticHandler := static.NewStaticFileHandler(location.Root, location.Index)
				handler = http.HandlerFunc(staticHandler.Handle)
			}

			// Register the handler for the path
			mux.Handle(location.Path, handler)
		}

		// Create the HTTP server
		httpServer := &http.Server{
			Addr:    serverConfig.Listen,
			Handler: mux,
		}

		s.servers = append(s.servers, httpServer)

		// Start the server in a goroutine
		go func() {
			log.Printf("Starting server on %s", serverConfig.Listen)
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("Server error on %s: %v", serverConfig.Listen, err)
			}
		}()
	}

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down servers...")

	// Create a deadline to wait for
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown all servers
	for _, server := range s.servers {
		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server forced to shutdown: %v", err)
		}
	}

	log.Println("Server exiting")
	return nil
}

func (s *Server) ReloadConfig(config *config.Config) error {
	// Validate new configuration
	if err := config.ValidateConfig(); err != nil {
		return fmt.Errorf("new configuration validation failed: %v", err)
	}

	// Update the internal config reference
	s.config = config
	log.Println("Configuration reloaded")
	return nil
}

func (s *Server) watchConfigFile(configPath string, reloadFunc func()) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	err = watcher.Add(configPath)
	if err != nil {
		log.Fatalf("Failed to watch config file: %v", err)
	}

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Println("Config file changed, reloading...")
				reloadFunc()
			}
		case err := <-watcher.Errors:
			log.Printf("Watcher error: %v", err)
		}
	}
}

func (s *Server) StartWithWatcher(configPath string) error {
	var mu sync.Mutex
	reloadFunc := func() {
		mu.Lock()
		defer mu.Unlock()
		newConfig, err := config.LoadConfig(configPath)
		if err != nil {
			log.Printf("Failed to reload configuration: %v", err)
			return
		}
		if err := s.ReloadConfig(newConfig); err != nil {
			log.Printf("Failed to apply new configuration: %v", err)
		}
	}

	go s.watchConfigFile(configPath, reloadFunc)

	return s.Start()
}
