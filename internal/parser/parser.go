package parser

import (
	"fmt"
	"strings"
)

// ComposeService represents a parsed docker run command as a compose service
type ComposeService struct {
	Image         string            `json:"image" yaml:"image"`
	ContainerName string            `json:"container_name,omitempty" yaml:"container_name,omitempty"`
	Hostname      string            `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	Restart       string            `json:"restart,omitempty" yaml:"restart,omitempty"`
	Ports         []string          `json:"ports,omitempty" yaml:"ports,omitempty"`
	Volumes       []string          `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	Environment   []string          `json:"environment,omitempty" yaml:"environment,omitempty"`
	Networks      []string          `json:"networks,omitempty" yaml:"networks,omitempty"`
	Command       string            `json:"command,omitempty" yaml:"command,omitempty"`
	Entrypoint    string            `json:"entrypoint,omitempty" yaml:"entrypoint,omitempty"`
	User          string            `json:"user,omitempty" yaml:"user,omitempty"`
	WorkDir       string            `json:"working_dir,omitempty" yaml:"working_dir,omitempty"`
	Privileged    bool              `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	NetworkMode   string            `json:"network_mode,omitempty" yaml:"network_mode,omitempty"`
	DNS           []string          `json:"dns,omitempty" yaml:"dns,omitempty"`
	DNSSearch     []string          `json:"dns_search,omitempty" yaml:"dns_search,omitempty"`
	ExtraHosts    []string          `json:"extra_hosts,omitempty" yaml:"extra_hosts,omitempty"`
	CapAdd        []string          `json:"cap_add,omitempty" yaml:"cap_add,omitempty"`
	CapDrop       []string          `json:"cap_drop,omitempty" yaml:"cap_drop,omitempty"`
	Devices       []string          `json:"devices,omitempty" yaml:"devices,omitempty"`
	Labels        map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	MemLimit      string            `json:"mem_limit,omitempty" yaml:"mem_limit,omitempty"`
	CPUShares     int64             `json:"cpu_shares,omitempty" yaml:"cpu_shares,omitempty"`
	CPUs          string            `json:"cpus,omitempty" yaml:"cpus,omitempty"`
	PidMode       string            `json:"pid,omitempty" yaml:"pid,omitempty"`
	ShmSize       string            `json:"shm_size,omitempty" yaml:"shm_size,omitempty"`
	Ulimits       map[string]string `json:"ulimits,omitempty" yaml:"ulimits,omitempty"`
	Sysctls       map[string]string `json:"sysctls,omitempty" yaml:"sysctls,omitempty"`
	SecurityOpt   []string          `json:"security_opt,omitempty" yaml:"security_opt,omitempty"`
	Init          bool              `json:"init,omitempty" yaml:"init,omitempty"`
	ReadOnly      bool              `json:"read_only,omitempty" yaml:"read_only,omitempty"`
	StopSignal    string            `json:"stop_signal,omitempty" yaml:"stop_signal,omitempty"`
	LogDriver     string            `json:"logging_driver,omitempty" yaml:"logging_driver,omitempty"`
	LogOptions    map[string]string `json:"logging_options,omitempty" yaml:"logging_options,omitempty"`
	EnvFile       []string          `json:"env_file,omitempty" yaml:"env_file,omitempty"`
	Tmpfs         []string          `json:"tmpfs,omitempty" yaml:"tmpfs,omitempty"`
	GroupAdd      []string          `json:"group_add,omitempty" yaml:"group_add,omitempty"`
	Runtime       string            `json:"runtime,omitempty" yaml:"runtime,omitempty"`
	Platform      string            `json:"platform,omitempty" yaml:"platform,omitempty"`
	IPAddress     string            `json:"ip,omitempty" yaml:"ip,omitempty"`
	MacAddress    string            `json:"mac_address,omitempty" yaml:"mac_address,omitempty"`
	VolumesFrom   []string          `json:"volumes_from,omitempty" yaml:"volumes_from,omitempty"`
}

// ParseDockerRun parses a docker run command into a ComposeService
func ParseDockerRun(cmd string) (*ComposeService, error) {
	cmd = strings.TrimSpace(cmd)
	// remove leading "docker run" or "docker container run"
	for _, prefix := range []string{"docker container run", "docker run"} {
		if strings.HasPrefix(cmd, prefix) {
			cmd = strings.TrimSpace(cmd[len(prefix):])
			break
		}
	}

	tokens, err := tokenize(cmd)
	if err != nil {
		return nil, err
	}

	svc := &ComposeService{
		Labels:     make(map[string]string),
		LogOptions: make(map[string]string),
		Sysctls:    make(map[string]string),
		Ulimits:    make(map[string]string),
	}

	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		switch tok {
		case "--detach", "-d":
			// ignore
		case "--rm":
			// ignore in compose context
		case "--tty", "-t", "--interactive", "-i", "-it", "-ti":
			// ignore
		case "--name":
			i++
			if i < len(tokens) {
				svc.ContainerName = tokens[i]
			}
		case "--hostname", "-h":
			i++
			if i < len(tokens) {
				svc.Hostname = tokens[i]
			}
		case "--restart":
			i++
			if i < len(tokens) {
				svc.Restart = tokens[i]
			}
		case "-p", "--publish":
			i++
			if i < len(tokens) {
				svc.Ports = append(svc.Ports, tokens[i])
			}
		case "-P", "--publish-all":
			// no direct compose equivalent
		case "-v", "--volume":
			i++
			if i < len(tokens) {
				svc.Volumes = append(svc.Volumes, tokens[i])
			}
		case "--mount":
			i++
			if i < len(tokens) {
				svc.Volumes = append(svc.Volumes, convertMount(tokens[i]))
			}
		case "-e", "--env":
			i++
			if i < len(tokens) {
				svc.Environment = append(svc.Environment, tokens[i])
			}
		case "--env-file":
			i++
			if i < len(tokens) {
				svc.EnvFile = append(svc.EnvFile, tokens[i])
			}
		case "--network", "--net":
			i++
			if i < len(tokens) {
				n := tokens[i]
				if n == "host" || n == "none" || strings.HasPrefix(n, "container:") {
					svc.NetworkMode = n
				} else {
					svc.Networks = append(svc.Networks, n)
				}
			}
		case "--network-alias":
			i++
			// skip, no direct compose equivalent at service level
		case "--ip":
			i++
			if i < len(tokens) {
				svc.IPAddress = tokens[i]
			}
		case "--mac-address":
			i++
			if i < len(tokens) {
				svc.MacAddress = tokens[i]
			}
		case "--dns":
			i++
			if i < len(tokens) {
				svc.DNS = append(svc.DNS, tokens[i])
			}
		case "--dns-search":
			i++
			if i < len(tokens) {
				svc.DNSSearch = append(svc.DNSSearch, tokens[i])
			}
		case "--add-host":
			i++
			if i < len(tokens) {
				svc.ExtraHosts = append(svc.ExtraHosts, tokens[i])
			}
		case "--entrypoint":
			i++
			if i < len(tokens) {
				svc.Entrypoint = tokens[i]
			}
		case "-u", "--user":
			i++
			if i < len(tokens) {
				svc.User = tokens[i]
			}
		case "-w", "--workdir":
			i++
			if i < len(tokens) {
				svc.WorkDir = tokens[i]
			}
		case "--privileged":
			svc.Privileged = true
		case "--cap-add":
			i++
			if i < len(tokens) {
				svc.CapAdd = append(svc.CapAdd, tokens[i])
			}
		case "--cap-drop":
			i++
			if i < len(tokens) {
				svc.CapDrop = append(svc.CapDrop, tokens[i])
			}
		case "--device":
			i++
			if i < len(tokens) {
				svc.Devices = append(svc.Devices, tokens[i])
			}
		case "-l", "--label":
			i++
			if i < len(tokens) {
				parts := strings.SplitN(tokens[i], "=", 2)
				if len(parts) == 2 {
					svc.Labels[parts[0]] = parts[1]
				} else {
					svc.Labels[parts[0]] = ""
				}
			}
		case "-m", "--memory":
			i++
			if i < len(tokens) {
				svc.MemLimit = tokens[i]
			}
		case "--cpus":
			i++
			if i < len(tokens) {
				svc.CPUs = tokens[i]
			}
		case "--cpu-shares":
			i++
			// skip exact parsing
		case "--pid":
			i++
			if i < len(tokens) {
				svc.PidMode = tokens[i]
			}
		case "--shm-size":
			i++
			if i < len(tokens) {
				svc.ShmSize = tokens[i]
			}
		case "--ulimit":
			i++
			if i < len(tokens) {
				parts := strings.SplitN(tokens[i], "=", 2)
				if len(parts) == 2 {
					svc.Ulimits[parts[0]] = parts[1]
				}
			}
		case "--sysctl":
			i++
			if i < len(tokens) {
				parts := strings.SplitN(tokens[i], "=", 2)
				if len(parts) == 2 {
					svc.Sysctls[parts[0]] = parts[1]
				}
			}
		case "--security-opt":
			i++
			if i < len(tokens) {
				svc.SecurityOpt = append(svc.SecurityOpt, tokens[i])
			}
		case "--init":
			svc.Init = true
		case "--read-only":
			svc.ReadOnly = true
		case "--stop-signal":
			i++
			if i < len(tokens) {
				svc.StopSignal = tokens[i]
			}
		case "--log-driver":
			i++
			if i < len(tokens) {
				svc.LogDriver = tokens[i]
			}
		case "--log-opt":
			i++
			if i < len(tokens) {
				parts := strings.SplitN(tokens[i], "=", 2)
				if len(parts) == 2 {
					svc.LogOptions[parts[0]] = parts[1]
				}
			}
		case "--tmpfs":
			i++
			if i < len(tokens) {
				svc.Tmpfs = append(svc.Tmpfs, tokens[i])
			}
		case "--group-add":
			i++
			if i < len(tokens) {
				svc.GroupAdd = append(svc.GroupAdd, tokens[i])
			}
		case "--runtime":
			i++
			if i < len(tokens) {
				svc.Runtime = tokens[i]
			}
		case "--platform":
			i++
			if i < len(tokens) {
				svc.Platform = tokens[i]
			}
		case "--volumes-from":
			i++
			if i < len(tokens) {
				svc.VolumesFrom = append(svc.VolumesFrom, tokens[i])
			}
		case "--link":
			i++
			// skip: deprecated in compose
		case "--expose":
			i++
			// skip: handled by ports
		default:
			// Handle --flag=value style
			if strings.HasPrefix(tok, "--") || strings.HasPrefix(tok, "-") {
				if strings.Contains(tok, "=") {
					parts := strings.SplitN(tok, "=", 2)
					// re-process as separate flag + value
					tokens = append([]string{parts[0], parts[1]}, tokens[i+1:]...)
					continue
				}
			} else if svc.Image == "" {
				svc.Image = tok
			} else {
				// remaining args are command
				rest := strings.Join(tokens[i:], " ")
				svc.Command = rest
				break
			}
		}
		i++
	}

	if svc.Image == "" {
		return nil, fmt.Errorf("no image specified in docker run command")
	}

	// cleanup empty maps
	if len(svc.Labels) == 0 {
		svc.Labels = nil
	}
	if len(svc.LogOptions) == 0 {
		svc.LogOptions = nil
	}
	if len(svc.Sysctls) == 0 {
		svc.Sysctls = nil
	}
	if len(svc.Ulimits) == 0 {
		svc.Ulimits = nil
	}

	return svc, nil
}

// ToYAML converts service to compose YAML string
func (s *ComposeService) ToYAML() string {
	name := s.ContainerName
	if name == "" {
		// derive name from image
		parts := strings.Split(s.Image, "/")
		last := parts[len(parts)-1]
		name = strings.Split(last, ":")[0]
	}

	var sb strings.Builder
	sb.WriteString("version: '3.8'\n\nservices:\n")
	sb.WriteString(fmt.Sprintf("  %s:\n", name))
	sb.WriteString(fmt.Sprintf("    image: %s\n", s.Image))

	if s.ContainerName != "" {
		sb.WriteString(fmt.Sprintf("    container_name: %s\n", s.ContainerName))
	}
	if s.Hostname != "" {
		sb.WriteString(fmt.Sprintf("    hostname: %s\n", s.Hostname))
	}
	if s.Restart != "" {
		sb.WriteString(fmt.Sprintf("    restart: %s\n", s.Restart))
	}
	if s.Privileged {
		sb.WriteString("    privileged: true\n")
	}
	if s.NetworkMode != "" {
		sb.WriteString(fmt.Sprintf("    network_mode: %s\n", s.NetworkMode))
	}
	if s.User != "" {
		sb.WriteString(fmt.Sprintf("    user: %s\n", s.User))
	}
	if s.WorkDir != "" {
		sb.WriteString(fmt.Sprintf("    working_dir: %s\n", s.WorkDir))
	}
	if s.Entrypoint != "" {
		sb.WriteString(fmt.Sprintf("    entrypoint: %s\n", s.Entrypoint))
	}
	if s.Command != "" {
		sb.WriteString(fmt.Sprintf("    command: %s\n", s.Command))
	}
	if s.MemLimit != "" {
		sb.WriteString(fmt.Sprintf("    mem_limit: %s\n", s.MemLimit))
	}
	if s.CPUs != "" {
		sb.WriteString(fmt.Sprintf("    cpus: %s\n", s.CPUs))
	}
	if s.ShmSize != "" {
		sb.WriteString(fmt.Sprintf("    shm_size: %s\n", s.ShmSize))
	}
	if s.Init {
		sb.WriteString("    init: true\n")
	}
	if s.ReadOnly {
		sb.WriteString("    read_only: true\n")
	}
	if s.StopSignal != "" {
		sb.WriteString(fmt.Sprintf("    stop_signal: %s\n", s.StopSignal))
	}
	if s.Runtime != "" {
		sb.WriteString(fmt.Sprintf("    runtime: %s\n", s.Runtime))
	}
	if s.Platform != "" {
		sb.WriteString(fmt.Sprintf("    platform: %s\n", s.Platform))
	}
	if s.MacAddress != "" {
		sb.WriteString(fmt.Sprintf("    mac_address: %s\n", s.MacAddress))
	}
	if s.PidMode != "" {
		sb.WriteString(fmt.Sprintf("    pid: %s\n", s.PidMode))
	}

	writeList := func(key string, items []string) {
		if len(items) == 0 {
			return
		}
		sb.WriteString(fmt.Sprintf("    %s:\n", key))
		for _, item := range items {
			sb.WriteString(fmt.Sprintf("      - %s\n", item))
		}
	}

	writeList("ports", s.Ports)
	writeList("volumes", s.Volumes)
	writeList("environment", s.Environment)
	writeList("env_file", s.EnvFile)
	writeList("networks", s.Networks)
	writeList("dns", s.DNS)
	writeList("dns_search", s.DNSSearch)
	writeList("extra_hosts", s.ExtraHosts)
	writeList("cap_add", s.CapAdd)
	writeList("cap_drop", s.CapDrop)
	writeList("devices", s.Devices)
	writeList("security_opt", s.SecurityOpt)
	writeList("tmpfs", s.Tmpfs)
	writeList("group_add", s.GroupAdd)
	writeList("volumes_from", s.VolumesFrom)

	if len(s.Labels) > 0 {
		sb.WriteString("    labels:\n")
		for k, v := range s.Labels {
			sb.WriteString(fmt.Sprintf("      %s: %s\n", k, v))
		}
	}

	if len(s.Sysctls) > 0 {
		sb.WriteString("    sysctls:\n")
		for k, v := range s.Sysctls {
			sb.WriteString(fmt.Sprintf("      %s: %s\n", k, v))
		}
	}

	if s.LogDriver != "" || len(s.LogOptions) > 0 {
		sb.WriteString("    logging:\n")
		if s.LogDriver != "" {
			sb.WriteString(fmt.Sprintf("      driver: %s\n", s.LogDriver))
		}
		if len(s.LogOptions) > 0 {
			sb.WriteString("      options:\n")
			for k, v := range s.LogOptions {
				sb.WriteString(fmt.Sprintf("        %s: %s\n", k, v))
			}
		}
	}

	if len(s.Networks) > 0 {
		sb.WriteString("\nnetworks:\n")
		for _, n := range s.Networks {
			sb.WriteString(fmt.Sprintf("  %s:\n    external: true\n", n))
		}
	}

	return sb.String()
}

func convertMount(mountStr string) string {
	// convert --mount type=bind,source=/host,target=/container to /host:/container
	parts := strings.Split(mountStr, ",")
	var source, target, mtype string
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "source", "src":
			source = kv[1]
		case "target", "dst", "destination":
			target = kv[1]
		case "type":
			mtype = kv[1]
		}
	}
	if mtype == "tmpfs" {
		return target
	}
	if source != "" && target != "" {
		return source + ":" + target
	}
	return mountStr
}

// tokenize splits a shell command respecting quotes
func tokenize(cmd string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]
		switch {
		case ch == '\\' && !inSingle && i+1 < len(cmd):
			i++
			current.WriteByte(cmd[i])
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case (ch == ' ' || ch == '\t' || ch == '\n' || ch == '\\') && !inSingle && !inDouble:
			if ch == '\\' && i+1 < len(cmd) && cmd[i+1] == '\n' {
				i++ // skip line continuation
				continue
			}
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unclosed quote in command")
	}
	return tokens, nil
}
