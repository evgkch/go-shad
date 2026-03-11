//go:build !solution

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type FirewallRule struct {
	Endpoint               string   `yaml:"endpoint"`
	ForbiddenUserAgents    []string `yaml:"forbidden_user_agents,omitempty"`
	ForbiddenHeaders       []string `yaml:"forbidden_headers,omitempty"`
	RequiredHeaders        []string `yaml:"required_headers,omitempty"`
	MaxRequestLengthBytes  int      `yaml:"max_request_length_bytes,omitempty"`
	MaxResponseLengthBytes int      `yaml:"max_response_length_bytes,omitempty"`
	ForbiddenResponseCodes []int    `yaml:"forbidden_response_codes,omitempty"`
	ForbiddenRequestRe     []string `yaml:"forbidden_request_re,omitempty"`
	ForbiddenResponseRe    []string `yaml:"forbidden_response_re,omitempty"`
}

type FirewallConfig struct {
	Rules []FirewallRule `yaml:"rules"`
}

func loadConfig(path string) (*FirewallConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	var cfg FirewallConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse yaml: %w", err)
	}
	return &cfg, nil
}

type headerRule struct {
	name  string
	value string // если пустой — проверяем только наличие
}

func parseHeaderRule(s string) headerRule {
	parts := strings.SplitN(s, ": ", 2)
	if len(parts) == 2 {
		return headerRule{name: parts[0], value: parts[1]}
	}
	return headerRule{name: s}
}

type compiledRule struct {
	rule                FirewallRule
	forbiddenUserAgents []*regexp.Regexp
	forbiddenHeaders    []headerRule
	requiredHeaders     []headerRule
	forbiddenRequestRe  []*regexp.Regexp
	forbiddenResponseRe []*regexp.Regexp
}

func compileRegexps(patterns []string) ([]*regexp.Regexp, error) {
	var res []*regexp.Regexp
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp %q: %w", p, err)
		}
		res = append(res, re)
	}
	return res, nil
}

func compileRule(r FirewallRule) (*compiledRule, error) {
	cr := &compiledRule{rule: r}

	var err error
	cr.forbiddenUserAgents, err = compileRegexps(r.ForbiddenUserAgents)
	if err != nil {
		return nil, err
	}
	cr.forbiddenRequestRe, err = compileRegexps(r.ForbiddenRequestRe)
	if err != nil {
		return nil, err
	}
	cr.forbiddenResponseRe, err = compileRegexps(r.ForbiddenResponseRe)
	if err != nil {
		return nil, err
	}

	for _, h := range r.ForbiddenHeaders {
		cr.forbiddenHeaders = append(cr.forbiddenHeaders, parseHeaderRule(h))
	}
	for _, h := range r.RequiredHeaders {
		cr.requiredHeaders = append(cr.requiredHeaders, parseHeaderRule(h))
	}

	return cr, nil
}

func (cr *compiledRule) checkRequest(r *http.Request) bool {
	// forbidden user agents (regexp)
	ua := r.Header.Get("User-Agent")
	for _, re := range cr.forbiddenUserAgents {
		if re.MatchString(ua) {
			return true
		}
	}

	for _, h := range cr.forbiddenHeaders {
		val := r.Header.Get(h.name)
		if h.value == "" && val != "" {
			return true
		}
		if h.value != "" && val == h.value {
			return true
		}
	}

	// required headers
	for _, h := range cr.requiredHeaders {
		val := r.Header.Get(h.name)
		if val == "" {
			return true
		}
		if h.value != "" && val != h.value {
			return true
		}
	}

	// max request length
	if cr.rule.MaxRequestLengthBytes > 0 && r.ContentLength > int64(cr.rule.MaxRequestLengthBytes) {
		return true
	}

	// forbidden request body regexps
	if len(cr.forbiddenRequestRe) > 0 && r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return false
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		for _, re := range cr.forbiddenRequestRe {
			if re.Match(body) {
				return true
			}
		}
	}

	return false
}

func (cr *compiledRule) checkResponse(resp *http.Response) bool {
	// forbidden response codes
	for _, code := range cr.rule.ForbiddenResponseCodes {
		if resp.StatusCode == code {
			return true
		}
	}

	// max response length
	if cr.rule.MaxResponseLengthBytes > 0 && resp.ContentLength > int64(cr.rule.MaxResponseLengthBytes) {
		return true
	}

	// forbidden response body regexps
	if len(cr.forbiddenResponseRe) > 0 && resp.Body != nil {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return false
		}
		resp.Body = io.NopCloser(bytes.NewReader(body))
		for _, re := range cr.forbiddenResponseRe {
			if re.Match(body) {
				return true
			}
		}
	}

	return false
}

func forbidden(w http.ResponseWriter) {
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte("Forbidden"))
}

func main() {
	serviceAddr := flag.String("service-addr", "", "service address")
	addr := flag.String("addr", "", "firewall address")
	conf := flag.String("conf", "", "firewall config")
	flag.Parse()

	cfg, err := loadConfig(*conf)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	target, err := url.Parse(*serviceAddr)
	if err != nil {
		log.Fatalf("invalid service address: %v", err)
	}

	client := &http.Client{}
	router := http.NewServeMux()

	for _, rule := range cfg.Rules {
		cr, err := compileRule(rule)
		if err != nil {
			log.Fatalf("failed to compile rule: %v", err)
		}

		router.HandleFunc(cr.rule.Endpoint, func(w http.ResponseWriter, r *http.Request) {
			if cr.checkRequest(r) {
				forbidden(w)
				return
			}

			r.URL.Host = target.Host
			r.URL.Scheme = target.Scheme
			r.RequestURI = ""

			resp, err := client.Do(r)
			if err != nil {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()

			if cr.checkResponse(resp) {
				forbidden(w)
				return
			}

			for k, v := range resp.Header {
				w.Header()[k] = v
			}
			w.WriteHeader(resp.StatusCode)
			io.Copy(w, resp.Body)
		})
	}

	// проксируем все что не попало под правила
	hasCatchAll := false
	for _, rule := range cfg.Rules {
		if rule.Endpoint == "/" {
			hasCatchAll = true
			break
		}
	}
	if !hasCatchAll {
		router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			r.URL.Host = target.Host
			r.URL.Scheme = target.Scheme
			r.RequestURI = ""

			resp, err := client.Do(r)
			if err != nil {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()

			for k, v := range resp.Header {
				w.Header()[k] = v
			}
			w.WriteHeader(resp.StatusCode)
			io.Copy(w, resp.Body)
		})
	}

	log.Printf("Firewall listening on %s, proxying to %s", *addr, *serviceAddr)
	if err := http.ListenAndServe(*addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
