package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

// ServerConfig represents a server block configuration
type ServerConfig struct {
	Listen     string           `yaml:"listen"`
	ServerName string           `yaml:"server_name"`
	Locations  []LocationConfig `yaml:"locations"`
}

// LocationConfig represents a location block configuration
type LocationConfig struct {
	Path             string            `yaml:"path"`
	ProxyPass        string            `yaml:"proxy_pass"`
	Root             string            `yaml:"root"`
	Index            string            `yaml:"index"`
	ProxySet         map[string]string `yaml:"proxy_set"`
	ProxyPassHeaders []string          `yaml:"proxy_pass_headers"`
}

// UpstreamConfig represents an upstream server group
type UpstreamConfig struct {
	Name    string   `yaml:"name"`
	Servers []string `yaml:"servers"`
}

// Config represents the main configuration structure
type Config struct {
	Servers   []ServerConfig   `yaml:"servers"`
	Upstreams []UpstreamConfig `yaml:"upstreams"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	return &config, nil
}

// GetUpstreamByName returns an upstream configuration by name
func (c *Config) GetUpstreamByName(name string) *UpstreamConfig {
	for _, upstream := range c.Upstreams {
		if upstream.Name == name {
			return &upstream
		}
	}
	return nil
}

// ValidateConfig validates the configuration
func (c *Config) ValidateConfig() error {
	serverNames := make(map[string]bool)
	for _, server := range c.Servers {
		if server.Listen == "" {
			return fmt.Errorf("server listen address cannot be empty")
		}

		if server.ServerName != "" {
			if serverNames[server.ServerName] {
				return fmt.Errorf("duplicate server_name found: %s", server.ServerName)
			}
			serverNames[server.ServerName] = true
		}

		for _, location := range server.Locations {
			if location.Path == "" {
				return fmt.Errorf("location path cannot be empty")
			}

			if location.ProxyPass != "" && location.Root != "" {
				return fmt.Errorf("location %s cannot have both proxy_pass and root", location.Path)
			}

			if location.ProxyPass == "" && location.Root == "" {
				return fmt.Errorf("location %s must have either proxy_pass or root", location.Path)
			}

			if location.ProxyPass != "" {
				// Check if it's a direct URL or an upstream reference
				if strings.HasPrefix(location.ProxyPass, "http://") || strings.HasPrefix(location.ProxyPass, "https://") {
					// This is a direct URL, validate it as such if needed
					// For now, we'll just accept valid HTTP/HTTPS URLs
				} else {
					// This is an upstream reference, validate that the upstream exists
					upstream := c.GetUpstreamByName(location.ProxyPass)
					if upstream == nil {
						return fmt.Errorf("upstream '%s' not found for location '%s'", location.ProxyPass, location.Path)
					}
				}
			}

			if location.Root != "" {
				if _, err := os.Stat(location.Root); os.IsNotExist(err) {
					return fmt.Errorf("root directory does not exist: %s", location.Root)
				}
			}
		}
	}

	return nil
}

// PrintConfig prints the configuration for debugging
func (c *Config) PrintConfig() {
	log.Printf("Loaded configuration:")
	log.Printf("  Servers: %d", len(c.Servers))
	log.Printf("  Upstreams: %d", len(c.Upstreams))

	for i, server := range c.Servers {
		log.Printf("  Server %d: listen=%s, server_name=%s", i, server.Listen, server.ServerName)
		for j, location := range server.Locations {
			log.Printf("    Location %d: path=%s", j, location.Path)
			if location.ProxyPass != "" {
				log.Printf("      proxy_pass=%s", location.ProxyPass)
			}
			if location.Root != "" {
				log.Printf("      root=%s", location.Root)
			}
		}
	}

	for i, upstream := range c.Upstreams {
		log.Printf("  Upstream %d: name=%s, servers=%v", i, upstream.Name, upstream.Servers)
	}
}
