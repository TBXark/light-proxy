package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
)

type Config struct {
	Address   string            `json:"address"`
	LocalLog  string            `json:"local_log"`
	Locations []*LocationConfig `json:"location"`
}

type LocationConfig struct {
	Path    string        `json:"path"`
	Target  string        `json:"target"`
	Rewrite *PathRwConfig `json:"rewrite"`
}

type PathRwConfig struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func initConfig() Config {
	var configPath string
	flag.StringVar(&configPath, "c", "config.json", "config file path")
	flag.Parse()

	file, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatal(err)
	}

	var config Config
	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Fatal(err)
	}

	if config.LocalLog != "" {
		logFile, err := os.OpenFile(config.LocalLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
		}
		log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	}
	return config
}

func createProxyServer(config *Config) {
	pathURLMap := make(map[string]*url.URL)
	for _, cfg := range config.Locations {
		target, err := url.Parse(cfg.Target)
		if err != nil {
			log.Fatal(err)
		}
		pathURLMap[cfg.Path] = target
	}

	regexpMap := make(map[string]*regexp.Regexp)
	for _, cfg := range config.Locations {
		if cfg.Rewrite != nil {
			regexpMap[cfg.Rewrite.From] = regexp.MustCompile(cfg.Rewrite.From)
		}
	}

	rp := &httputil.ReverseProxy{
		Director: func(request *http.Request) {
			for _, cfg := range config.Locations {
				if strings.HasPrefix(request.URL.Path, cfg.Path) {
					target := pathURLMap[cfg.Path]
					request.Host = target.Host
					request.URL.Scheme = target.Scheme
					request.URL.Host = target.Host
					request.Header.Set("X-Forwarded-Host", request.Header.Get("Host"))
					request.Header.Set("X-Origin-Host", target.Host)
					if cfg.Rewrite != nil {
						request.URL.Path = regexpMap[cfg.Rewrite.From].ReplaceAllString(request.URL.Path, cfg.Rewrite.To)
					}
					log.Printf("[%6s] %s", request.Method, request.URL.Path)
					break
				}
			}
		},
	}
	log.Printf("Starting proxy on port %s", config.Address)
	log.Fatal(http.ListenAndServe(config.Address, rp))
}

func main() {
	config := initConfig()
	createProxyServer(&config)
}
