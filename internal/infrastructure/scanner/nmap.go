package scanner

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"apant_be/internal/domain"
)

var (
	nmapTargetPattern = regexp.MustCompile(`^[a-zA-Z0-9./:-]+$`)
	nmapPortsPattern  = regexp.MustCompile(`^[0-9,-]+$`)
)

type DockerNmapConfig struct {
	DockerBinary string
	NmapImage    string
	Timeout      time.Duration
}

type DockerNmapExecutor struct {
	dockerBinary string
	nmapImage    string
	timeout      time.Duration
}

func NewDockerNmapExecutor(cfg DockerNmapConfig) *DockerNmapExecutor {
	dockerBinary := strings.TrimSpace(cfg.DockerBinary)
	if dockerBinary == "" {
		dockerBinary = "docker"
	}

	nmapImage := strings.TrimSpace(cfg.NmapImage)
	if nmapImage == "" {
		nmapImage = "instrumentisto/nmap:latest"
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	return &DockerNmapExecutor{dockerBinary: dockerBinary, nmapImage: nmapImage, timeout: timeout}
}

func (e *DockerNmapExecutor) Execute(intent *domain.ToolIntent) map[string]any {
	if intent == nil {
		return map[string]any{"status": "error", "error": "nil tool intent"}
	}

	name := strings.TrimSpace(strings.ToLower(intent.Name))
	if name != "nmap_scan" {
		return map[string]any{
			"status": "error",
			"tool":   name,
			"error":  "tool is not implemented by docker executor",
		}
	}

	return e.executeNmap(intent)
}

func (e *DockerNmapExecutor) executeNmap(intent *domain.ToolIntent) map[string]any {
	target, err := normalizeNmapTarget(intent.Params)
	if err != nil {
		return map[string]any{"status": "error", "tool": "nmap_scan", "error": err.Error()}
	}

	ports, err := parseNmapPorts(intent.Params)
	if err != nil {
		return map[string]any{"status": "error", "tool": "nmap_scan", "error": err.Error()}
	}

	topPorts, err := parseTopPorts(intent.Params)
	if err != nil {
		return map[string]any{"status": "error", "tool": "nmap_scan", "error": err.Error()}
	}

	serviceDetection, _ := intent.Params["service_detection"].(bool)

	nmapArgs := []string{"-Pn", "-n", "--max-retries", "1", "--host-timeout", "20s", "-oX", "-"}
	if ports != "" {
		nmapArgs = append(nmapArgs, "-p", ports)
	} else {
		nmapArgs = append(nmapArgs, "--top-ports", strconv.Itoa(topPorts))
	}
	if serviceDetection {
		nmapArgs = append(nmapArgs, "-sV")
	}
	nmapArgs = append(nmapArgs, target)

	dockerArgs := []string{"run", "--rm", "--network", "bridge", "--cap-drop", "ALL", e.nmapImage}
	dockerArgs = append(dockerArgs, nmapArgs...)

	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.dockerBinary, dockerArgs...)
	output, execErr := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return map[string]any{
			"status":          "error",
			"tool":            "nmap_scan",
			"error":           "nmap scan timed out",
			"timeout_seconds": int(e.timeout.Seconds()),
		}
	}

	if execErr != nil {
		return map[string]any{
			"status": "error",
			"tool":   "nmap_scan",
			"error":  fmt.Sprintf("failed to run docker nmap: %v", execErr),
			"stderr": strings.TrimSpace(string(output)),
		}
	}

	parsed, err := parseNmapXML(output)
	if err != nil {
		return map[string]any{
			"status": "error",
			"tool":   "nmap_scan",
			"error":  fmt.Sprintf("failed to parse nmap xml: %v", err),
			"output": strings.TrimSpace(string(output)),
		}
	}

	return map[string]any{
		"status":           "success",
		"tool":             "nmap_scan",
		"engine":           "docker",
		"image":            e.nmapImage,
		"target":           target,
		"service_detected": serviceDetection,
		"nmap_args":        nmapArgs,
		"open_port_count":  parsed.OpenPortCount,
		"hosts":            parsed.Hosts,
	}
}

func normalizeNmapTarget(params map[string]any) (string, error) {
	target, _ := params["target"].(string)
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("nmap_scan requires target")
	}

	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		u, err := url.Parse(target)
		if err != nil {
			return "", fmt.Errorf("invalid target url")
		}
		target = strings.TrimSpace(u.Hostname())
	}

	if target == "" || strings.ContainsAny(target, " \t\n\r") {
		return "", fmt.Errorf("target must be a hostname, ip, or cidr")
	}
	if len(target) > 255 {
		return "", fmt.Errorf("target is too long")
	}
	if !nmapTargetPattern.MatchString(target) {
		return "", fmt.Errorf("target has unsupported characters")
	}

	return target, nil
}

func parseNmapPorts(params map[string]any) (string, error) {
	ports, _ := params["ports"].(string)
	ports = strings.TrimSpace(ports)
	if ports == "" {
		return "", nil
	}

	if !nmapPortsPattern.MatchString(ports) {
		return "", fmt.Errorf("ports must only contain digits, comma, and dash")
	}

	return ports, nil
}

func parseTopPorts(params map[string]any) (int, error) {
	v, ok := params["top_ports"]
	if !ok {
		return 100, nil
	}

	var topPorts int
	switch val := v.(type) {
	case float64:
		topPorts = int(val)
	case int:
		topPorts = val
	default:
		return 0, fmt.Errorf("top_ports must be a number")
	}

	if topPorts < 1 || topPorts > 1000 {
		return 0, fmt.Errorf("top_ports must be between 1 and 1000")
	}

	return topPorts, nil
}

type parsedNmapResult struct {
	Hosts         []map[string]any
	OpenPortCount int
}

type nmapRunXML struct {
	XMLName xml.Name      `xml:"nmaprun"`
	Hosts   []nmapHostXML `xml:"host"`
}

type nmapHostXML struct {
	Status    nmapStatusXML     `xml:"status"`
	Addresses []nmapAddressXML  `xml:"address"`
	Hostnames []nmapHostnameXML `xml:"hostnames>hostname"`
	Ports     []nmapPortXML     `xml:"ports>port"`
}

type nmapStatusXML struct {
	State string `xml:"state,attr"`
}

type nmapAddressXML struct {
	Addr     string `xml:"addr,attr"`
	AddrType string `xml:"addrtype,attr"`
}

type nmapHostnameXML struct {
	Name string `xml:"name,attr"`
}

type nmapPortXML struct {
	Protocol string         `xml:"protocol,attr"`
	PortID   int            `xml:"portid,attr"`
	State    nmapPortState  `xml:"state"`
	Service  nmapServiceXML `xml:"service"`
}

type nmapPortState struct {
	State string `xml:"state,attr"`
}

type nmapServiceXML struct {
	Name    string `xml:"name,attr"`
	Product string `xml:"product,attr"`
	Version string `xml:"version,attr"`
	Extra   string `xml:"extrainfo,attr"`
}

func parseNmapXML(data []byte) (parsedNmapResult, error) {
	var run nmapRunXML
	if err := xml.Unmarshal(data, &run); err != nil {
		return parsedNmapResult{}, err
	}

	out := parsedNmapResult{Hosts: make([]map[string]any, 0, len(run.Hosts))}
	for _, host := range run.Hosts {
		address := primaryAddress(host.Addresses)
		hostname := ""
		if len(host.Hostnames) > 0 {
			hostname = strings.TrimSpace(host.Hostnames[0].Name)
		}

		ports := make([]map[string]any, 0, len(host.Ports))
		for _, p := range host.Ports {
			if strings.ToLower(strings.TrimSpace(p.State.State)) != "open" {
				continue
			}

			ports = append(ports, map[string]any{
				"protocol": p.Protocol,
				"port":     p.PortID,
				"state":    p.State.State,
				"service":  strings.TrimSpace(p.Service.Name),
				"product":  strings.TrimSpace(p.Service.Product),
				"version":  strings.TrimSpace(p.Service.Version),
				"extra":    strings.TrimSpace(p.Service.Extra),
			})
		}

		out.OpenPortCount += len(ports)
		out.Hosts = append(out.Hosts, map[string]any{
			"address":    address,
			"hostname":   hostname,
			"host_state": host.Status.State,
			"ports":      ports,
		})
	}

	return out, nil
}

func primaryAddress(addresses []nmapAddressXML) string {
	if len(addresses) == 0 {
		return ""
	}

	for _, a := range addresses {
		if strings.EqualFold(a.AddrType, "ipv4") {
			return strings.TrimSpace(a.Addr)
		}
	}

	return strings.TrimSpace(addresses[0].Addr)
}
