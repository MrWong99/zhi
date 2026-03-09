// zhi-config-ansible is a zhi configuration plugin that reads Ansible YAML
// inventory files and group_vars/host_vars directories, exposing them as a
// flat zhi configuration tree with write-back support.
//
// Tree mapping:
//
//	hosts/<hostname>/ansible_host      from inventory hosts
//	hosts/<hostname>/ansible_user      from inventory hosts
//	hosts/<hostname>/ansible_port      from inventory hosts
//	hosts/<hostname>/vars/<key>        from host_vars/<hostname>/*.yml
//	groups/<group>/members             comma-separated host list
//	groups/<group>/vars/<key>          from group_vars/<group>.yml or inventory group vars
package main

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"gopkg.in/yaml.v3"

	"github.com/MrWong99/zhi/pkg/zhiplugin"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

const maxFileSize = 10 << 20 // 10 MB

// groupNamePattern matches valid Ansible group names.
var groupNamePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// ansiblePlugin implements config.Plugin.
type ansiblePlugin struct {
	mu     sync.RWMutex
	values map[string]*config.Value

	// projectRoot is the directory containing the Ansible project.
	projectRoot string
	// inventoryPath is the path to the YAML inventory file.
	inventoryPath string
	// groupVarsDir and hostVarsDir are paths to var file directories.
	groupVarsDir string
	hostVarsDir  string

	logger hclog.Logger
}

func newAnsiblePlugin(opts map[string]any, logger hclog.Logger) (*ansiblePlugin, error) {
	if logger == nil {
		logger = hclog.NewNullLogger()
	}
	p := &ansiblePlugin{
		values: make(map[string]*config.Value),
		logger: logger,
	}

	// Parse options.
	if v, ok := opts["inventory"].(string); ok {
		p.inventoryPath = v
	}
	if v, ok := opts["group_vars_dir"].(string); ok {
		p.groupVarsDir = v
	}
	if v, ok := opts["host_vars_dir"].(string); ok {
		p.hostVarsDir = v
	}

	// Determine project root from inventory path.
	if p.inventoryPath != "" {
		p.projectRoot = filepath.Dir(filepath.Dir(p.inventoryPath))
	}
	if p.projectRoot == "" {
		p.projectRoot = "."
	}
	var err error
	p.projectRoot, err = filepath.Abs(p.projectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolving project root: %w", err)
	}

	// Load data.
	if p.inventoryPath != "" {
		if err := p.loadInventory(); err != nil {
			return nil, fmt.Errorf("loading inventory: %w", err)
		}
	}
	// Also accept ansible-cfg option (currently unused, reserved for future).
	if v, ok := opts["ansible_cfg"].(string); ok {
		_ = v // reserved
	}
	if p.groupVarsDir != "" {
		if err := p.loadVarFiles(p.groupVarsDir, "groups"); err != nil {
			return nil, fmt.Errorf("loading group vars: %w", err)
		}
	}
	if p.hostVarsDir != "" {
		if err := p.loadVarFiles(p.hostVarsDir, "hosts"); err != nil {
			return nil, fmt.Errorf("loading host vars: %w", err)
		}
	}

	return p, nil
}

func (p *ansiblePlugin) List(_ context.Context) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	paths := make([]string, 0, len(p.values))
	for k := range p.values {
		paths = append(paths, k)
	}
	sort.Strings(paths)
	return paths, nil
}

func (p *ansiblePlugin) Get(_ context.Context, path string) (config.Value, bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	v, ok := p.values[path]
	if !ok {
		return config.Value{}, false, nil
	}
	return *v, true, nil
}

func (p *ansiblePlugin) Set(_ context.Context, path string, v config.Value) error {
	if err := config.ValidatePath(path); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Determine target file for write-back.
	sourceFile := ""
	if v.Metadata != nil {
		if sf, ok := v.Metadata["ansible.source_file"].(string); ok {
			sourceFile = sf
		}
	}
	if existing, ok := p.values[path]; ok && sourceFile == "" && existing.Metadata != nil {
		if sf, ok := existing.Metadata["ansible.source_file"].(string); ok {
			sourceFile = sf
		}
	}

	// Derive convention path if no source file.
	if sourceFile == "" {
		sourceFile = p.deriveSourceFile(path)
	}

	// Path confinement: ensure the target file is within project root.
	if sourceFile != "" {
		absSource, err := filepath.Abs(sourceFile)
		if err != nil {
			return fmt.Errorf("resolving source file: %w", err)
		}
		if !strings.HasPrefix(absSource, p.projectRoot+string(filepath.Separator)) && absSource != p.projectRoot {
			return fmt.Errorf("source file %q is outside project root %q", sourceFile, p.projectRoot)
		}
	}

	// Update in-memory cache.
	if v.Metadata == nil {
		v.Metadata = make(map[string]any)
	}
	if sourceFile != "" {
		v.Metadata["ansible.source_file"] = sourceFile
	}
	p.values[path] = &v

	// Write back to source file if we have one.
	if sourceFile != "" {
		if err := p.flushSourceFile(sourceFile); err != nil {
			return fmt.Errorf("writing back to %s: %w", sourceFile, err)
		}
	}

	return nil
}

func (p *ansiblePlugin) Validate(_ context.Context, path string, tree config.TreeReader) ([]config.ValidationResult, error) {
	_, ok := tree.Get(path)
	if !ok {
		return nil, nil
	}

	var results []config.ValidationResult

	// Rule 1: Host references in groups must exist.
	if strings.HasSuffix(path, "/members") {
		parts := strings.SplitN(path, "/", 3)
		if len(parts) >= 3 && parts[0] == "groups" {
			v, _ := tree.Get(path)
			members := strings.Split(fmt.Sprintf("%v", v.Val), ",")
			for _, member := range members {
				member = strings.TrimSpace(member)
				if member == "" {
					continue
				}
				// Check if host exists by looking for any path starting with hosts/<member>/
				found := false
				for _, tp := range tree.List() {
					if strings.HasPrefix(tp, "hosts/"+member+"/") {
						found = true
						break
					}
				}
				if !found {
					results = append(results, config.ValidationResult{
						Severity: config.Blocking,
						Message:  fmt.Sprintf("group %q references host %q which does not exist", parts[1], member),
					})
				}
			}
		}
	}

	// Rule 2: ansible_port must be numeric 1-65535.
	if strings.HasSuffix(path, "/ansible_port") {
		v, _ := tree.Get(path)
		valid := false
		switch port := v.Val.(type) {
		case float64:
			if port == math.Trunc(port) && port >= 1 && port <= 65535 {
				valid = true
			}
		case string:
			// Try to parse as number — values from YAML may arrive as strings.
			var f float64
			if _, err := fmt.Sscanf(port, "%f", &f); err == nil {
				if f == math.Trunc(f) && f >= 1 && f <= 65535 {
					valid = true
				}
			}
		}
		if !valid {
			results = append(results, config.ValidationResult{
				Severity: config.Warning,
				Message:  fmt.Sprintf("ansible_port should be a number between 1 and 65535, got %v", v.Val),
			})
		}
	}

	// Rule 3: Group names must match Ansible's pattern (no hyphens).
	parts := strings.SplitN(path, "/", 3)
	if len(parts) >= 2 && parts[0] == "groups" {
		groupName := parts[1]
		if !groupNamePattern.MatchString(groupName) {
			results = append(results, config.ValidationResult{
				Severity: config.Warning,
				Message:  fmt.Sprintf("group name %q does not match Ansible pattern [a-zA-Z_][a-zA-Z0-9_]*", groupName),
			})
		}
	}

	// Rule 4: ansible_host should be a valid IP address or resolvable hostname.
	if strings.HasSuffix(path, "/ansible_host") {
		v, _ := tree.Get(path)
		host := fmt.Sprintf("%v", v.Val)
		if net.ParseIP(host) == nil {
			// Not an IP -- check if it looks like a valid hostname.
			if !isValidHostname(host) {
				results = append(results, config.ValidationResult{
					Severity: config.Warning,
					Message:  fmt.Sprintf("ansible_host %q is not a valid IP address or hostname", host),
				})
			}
		}
	}

	// Rule 5: ansible_user should be non-empty.
	if strings.HasSuffix(path, "/ansible_user") {
		v, _ := tree.Get(path)
		user := strings.TrimSpace(fmt.Sprintf("%v", v.Val))
		if user == "" {
			results = append(results, config.ValidationResult{
				Severity: config.Warning,
				Message:  "ansible_user should not be empty",
			})
		}
	}

	// Rule 6: Warn on hosts that belong to no group.
	if parts := strings.SplitN(path, "/", 3); len(parts) >= 2 && parts[0] == "hosts" && strings.HasSuffix(path, "/ansible_host") {
		hostname := parts[1]
		found := false
		for _, tp := range tree.List() {
			if strings.HasPrefix(tp, "groups/") && strings.HasSuffix(tp, "/members") {
				v, ok := tree.Get(tp)
				if !ok {
					continue
				}
				members := strings.Split(fmt.Sprintf("%v", v.Val), ",")
				for _, m := range members {
					if strings.TrimSpace(m) == hostname {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
		}
		if !found {
			results = append(results, config.ValidationResult{
				Severity: config.Info,
				Message:  fmt.Sprintf("host %q does not belong to any group", hostname),
			})
		}
	}

	return results, nil
}

// loadInventory detects the inventory format (YAML or INI) and delegates.
func (p *ansiblePlugin) loadInventory() error {
	info, err := os.Stat(p.inventoryPath)
	if err != nil {
		return fmt.Errorf("stat inventory: %w", err)
	}
	if info.Size() > maxFileSize {
		return fmt.Errorf("inventory file too large: %d bytes (max %d)", info.Size(), maxFileSize)
	}

	ext := strings.ToLower(filepath.Ext(p.inventoryPath))
	switch ext {
	case ".ini", ".cfg":
		return p.loadINIInventory()
	default:
		return p.loadYAMLInventory()
	}
}

// loadYAMLInventory parses a YAML inventory file and populates the cache.
func (p *ansiblePlugin) loadYAMLInventory() error {
	data, err := os.ReadFile(p.inventoryPath)
	if err != nil {
		return fmt.Errorf("reading inventory: %w", err)
	}

	var inventory map[string]any
	if err := yaml.Unmarshal(data, &inventory); err != nil {
		return fmt.Errorf("parsing inventory: %w", err)
	}

	// The root should be "all".
	allRaw, ok := inventory["all"]
	if !ok {
		return fmt.Errorf("inventory missing 'all' root key")
	}
	all, ok := allRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("inventory 'all' must be a map")
	}

	// Parse hosts under all.hosts.
	if hostsRaw, ok := all["hosts"]; ok {
		if hosts, ok := hostsRaw.(map[string]any); ok {
			for hostname, hostDataRaw := range hosts {
				p.loadHost(hostname, hostDataRaw)
			}
		}
	}

	// Parse groups under all.children.
	if childrenRaw, ok := all["children"]; ok {
		if children, ok := childrenRaw.(map[string]any); ok {
			for groupName, groupDataRaw := range children {
				p.loadGroup(groupName, groupDataRaw)
			}
		}
	}

	return nil
}

// loadINIInventory parses an Ansible INI-format inventory file.
//
// Supported sections:
//
//	[groupname]          -- host entries with optional key=value pairs
//	[groupname:vars]     -- group variables
//	[groupname:children] -- child group names (one per line)
//
// Host lines: hostname [key=value ...]
func (p *ansiblePlugin) loadINIInventory() error {
	f, err := os.Open(p.inventoryPath)
	if err != nil {
		return fmt.Errorf("opening inventory: %w", err)
	}
	defer f.Close()

	type groupData struct {
		hosts    []string
		hostVars map[string]map[string]string // hostname -> key -> value
		vars     map[string]string
		children []string
	}

	groups := make(map[string]*groupData)
	currentGroup := ""
	sectionType := "" // "", "vars", "children"

	getGroup := func(name string) *groupData {
		g, ok := groups[name]
		if !ok {
			g = &groupData{
				hostVars: make(map[string]map[string]string),
				vars:     make(map[string]string),
			}
			groups[name] = g
		}
		return g
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments.
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Section header: [name], [name:vars], [name:children].
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			header := line[1 : len(line)-1]
			if idx := strings.Index(header, ":"); idx != -1 {
				currentGroup = header[:idx]
				sectionType = header[idx+1:]
			} else {
				currentGroup = header
				sectionType = ""
			}
			getGroup(currentGroup) // ensure it exists
			continue
		}

		if currentGroup == "" {
			// Lines before any section belong to the implicit "ungrouped" group.
			currentGroup = "ungrouped"
			sectionType = ""
			getGroup(currentGroup)
		}

		g := getGroup(currentGroup)

		switch sectionType {
		case "vars":
			// key=value
			if k, v, ok := strings.Cut(line, "="); ok {
				g.vars[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		case "children":
			// child group name
			g.children = append(g.children, line)
		default:
			// Host line: hostname [key=value ...]
			fields := strings.Fields(line)
			hostname := fields[0]
			g.hosts = append(g.hosts, hostname)
			if len(fields) > 1 {
				if g.hostVars[hostname] == nil {
					g.hostVars[hostname] = make(map[string]string)
				}
				for _, kv := range fields[1:] {
					if k, v, ok := strings.Cut(kv, "="); ok {
						g.hostVars[hostname][k] = v
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanning inventory: %w", err)
	}

	// Convert parsed data into zhi tree paths.
	allHosts := make(map[string]map[string]string) // hostname -> merged vars

	for groupName, g := range groups {
		// Collect host entries and their inline variables.
		for _, hostname := range g.hosts {
			if allHosts[hostname] == nil {
				allHosts[hostname] = make(map[string]string)
			}
			for k, v := range g.hostVars[hostname] {
				allHosts[hostname][k] = v
			}
		}

		// Skip ungrouped for group paths if it has no explicit vars or children.
		if groupName == "ungrouped" && len(g.vars) == 0 && len(g.children) == 0 {
			continue
		}

		// Set group members.
		if len(g.hosts) > 0 {
			sort.Strings(g.hosts)
			path := "groups/" + groupName + "/members"
			if config.ValidatePath(path) == nil {
				p.values[path] = &config.Value{
					Val:      strings.Join(g.hosts, ","),
					Metadata: map[string]any{"ansible.source_file": p.inventoryPath},
				}
			}
		}

		// Set group vars.
		for key, val := range g.vars {
			path := "groups/" + groupName + "/vars/" + key
			if config.ValidatePath(path) != nil {
				p.logger.Warn("skipping invalid path", "path", path)
				continue
			}
			p.values[path] = &config.Value{
				Val:      val,
				Metadata: map[string]any{"ansible.source_file": p.inventoryPath},
			}
		}
	}

	// Set host-level paths.
	for hostname, vars := range allHosts {
		for key, val := range vars {
			path := "hosts/" + hostname + "/" + key
			if config.ValidatePath(path) != nil {
				p.logger.Warn("skipping invalid path", "path", path)
				continue
			}
			p.values[path] = &config.Value{
				Val:      val,
				Metadata: map[string]any{"ansible.source_file": p.inventoryPath},
			}
		}
	}

	return nil
}

func (p *ansiblePlugin) loadHost(hostname string, hostDataRaw any) {
	hostData, ok := hostDataRaw.(map[string]any)
	if !ok {
		return
	}
	for key, val := range hostData {
		path := "hosts/" + hostname + "/" + key
		if config.ValidatePath(path) != nil {
			p.logger.Warn("skipping invalid path", "path", path)
			continue
		}
		p.values[path] = &config.Value{
			Val:      fmt.Sprintf("%v", val),
			Metadata: map[string]any{"ansible.source_file": p.inventoryPath},
		}
	}
}

func (p *ansiblePlugin) loadGroup(groupName string, groupDataRaw any) {
	groupData, ok := groupDataRaw.(map[string]any)
	if !ok {
		return
	}

	// Extract host members.
	if hostsRaw, ok := groupData["hosts"]; ok {
		if hosts, ok := hostsRaw.(map[string]any); ok {
			members := make([]string, 0, len(hosts))
			for h := range hosts {
				members = append(members, h)
			}
			sort.Strings(members)
			path := "groups/" + groupName + "/members"
			if config.ValidatePath(path) == nil {
				p.values[path] = &config.Value{
					Val:      strings.Join(members, ","),
					Metadata: map[string]any{"ansible.source_file": p.inventoryPath},
				}
			}

			// Also load per-host data from group host entries.
			for h, hData := range hosts {
				p.loadHost(h, hData)
			}
		}
	}

	// Extract group vars.
	if varsRaw, ok := groupData["vars"]; ok {
		if vars, ok := varsRaw.(map[string]any); ok {
			for key, val := range vars {
				path := "groups/" + groupName + "/vars/" + key
				if config.ValidatePath(path) != nil {
					p.logger.Warn("skipping invalid path", "path", path)
					continue
				}
				p.values[path] = &config.Value{
					Val:      fmt.Sprintf("%v", val),
					Metadata: map[string]any{"ansible.source_file": p.inventoryPath},
				}
			}
		}
	}
}

// loadVarFiles scans a directory for YAML var files and loads them.
// For host_vars, the directory contains subdirectories per host.
// For group_vars, the directory contains files per group.
func (p *ansiblePlugin) loadVarFiles(dir, prefix string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading %s: %w", dir, err)
	}

	for _, entry := range entries {
		entryPath := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			// Subdirectory: scan for YAML files inside.
			subEntries, err := os.ReadDir(entryPath)
			if err != nil {
				return fmt.Errorf("reading %s: %w", entryPath, err)
			}
			for _, subEntry := range subEntries {
				if subEntry.IsDir() {
					continue
				}
				if isYAMLFile(subEntry.Name()) {
					if err := p.loadVarFile(filepath.Join(entryPath, subEntry.Name()), prefix, entry.Name()); err != nil {
						return err
					}
				}
			}
		} else if isYAMLFile(entry.Name()) {
			// Direct YAML file: name without extension is the entity name.
			name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			if err := p.loadVarFile(entryPath, prefix, name); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *ansiblePlugin) loadVarFile(filePath, prefix, entityName string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", filePath, err)
	}
	if info.Size() > maxFileSize {
		return fmt.Errorf("file too large: %s (%d bytes)", filePath, info.Size())
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", filePath, err)
	}

	var vars map[string]any
	if err := yaml.Unmarshal(data, &vars); err != nil {
		return fmt.Errorf("parsing %s: %w", filePath, err)
	}

	for key, val := range vars {
		path := prefix + "/" + entityName + "/vars/" + key
		if config.ValidatePath(path) != nil {
			p.logger.Warn("skipping invalid path from var file", "path", path, "file", filePath)
			continue
		}
		p.values[path] = &config.Value{
			Val:      fmt.Sprintf("%v", val),
			Metadata: map[string]any{"ansible.source_file": filePath},
		}
	}
	return nil
}

// deriveSourceFile returns the convention path for a new value based on its
// tree path. Returns empty string if the path doesn't map to a known pattern.
func (p *ansiblePlugin) deriveSourceFile(path string) string {
	parts := strings.SplitN(path, "/", 4)
	if len(parts) < 4 || parts[2] != "vars" {
		return ""
	}
	switch parts[0] {
	case "hosts":
		if p.hostVarsDir != "" {
			return filepath.Join(p.hostVarsDir, parts[1], "main.yml")
		}
	case "groups":
		if p.groupVarsDir != "" {
			return filepath.Join(p.groupVarsDir, parts[1]+".yml")
		}
	}
	return ""
}

// flushSourceFile writes all values with the given source file back to disk.
func (p *ansiblePlugin) flushSourceFile(sourceFile string) error {
	// Collect all values that belong to this source file.
	vars := make(map[string]any)
	for path, v := range p.values {
		if v.Metadata == nil {
			continue
		}
		sf, ok := v.Metadata["ansible.source_file"].(string)
		if !ok || sf != sourceFile {
			continue
		}
		// For var files, extract just the key name (last segment).
		parts := strings.SplitN(path, "/", 4)
		if len(parts) == 4 && parts[2] == "vars" {
			vars[parts[3]] = v.Val
		}
	}

	if len(vars) == 0 {
		return nil
	}

	data, err := yaml.Marshal(vars)
	if err != nil {
		return fmt.Errorf("marshalling vars: %w", err)
	}

	// Atomic write: temp file + rename.
	dir := filepath.Dir(sourceFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".zhi-ansible-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpName, sourceFile); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

func isYAMLFile(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".yml" || ext == ".yaml"
}

// hostnamePattern matches valid DNS hostnames (RFC 952 / RFC 1123).
var hostnamePattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?)*$`)

// isValidHostname checks whether s looks like a valid DNS hostname.
func isValidHostname(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	return hostnamePattern.MatchString(s)
}

func main() {
	level := hclog.LevelFromString(os.Getenv("ZHI_LOG_LEVEL"))
	if level == hclog.NoLevel {
		level = hclog.Info
	}
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "zhi-config-ansible",
		Level:  level,
		Output: os.Stderr,
	})
	logger.Info("starting ansible config plugin")

	// Plugin options are passed via environment.
	// In practice they come from the workspace config's options map.
	opts := map[string]any{}
	if v := os.Getenv("ZHI_PLUGIN_OPT_INVENTORY"); v != "" {
		opts["inventory"] = v
	}
	if v := os.Getenv("ZHI_PLUGIN_OPT_GROUP_VARS_DIR"); v != "" {
		opts["group_vars_dir"] = v
	}
	if v := os.Getenv("ZHI_PLUGIN_OPT_HOST_VARS_DIR"); v != "" {
		opts["host_vars_dir"] = v
	}

	plugin, err := newAnsiblePlugin(opts, logger)
	if err != nil {
		logger.Error("failed to initialize plugin", "error", err)
		os.Exit(1)
	}

	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: zhiplugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"config": &config.GRPCPlugin{Impl: plugin},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
		Logger:     logger,
	})
}
