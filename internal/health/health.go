package health

import (
	"fmt"
	"net/http"
	"time"
)

type Config struct {
	CoreURL   string
	EngineURL string
	ProxyURL  string
}

type Result struct {
	Service string
	OK      bool
	Message string
}

var client = &http.Client{Timeout: 5 * time.Second}

func Check(cfg Config) []Result {
	return []Result{
		probe("core", cfg.CoreURL+"/health"),
		probe("engine", cfg.EngineURL+"/health"),
		probe("proxy", cfg.ProxyURL+"/health"),
	}
}

func probe(service, url string) Result {
	resp, err := client.Get(url)
	if err != nil {
		return Result{Service: service, OK: false, Message: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{Service: service, OK: false, Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	return Result{Service: service, OK: true, Message: "ok"}
}
