package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/mod/modfile"
	"sigs.k8s.io/yaml"
)

const (
	modFile = "./go.mod"
)

type config struct {
	UpstreamRefs    []string `json:"upstreamRefs"`
	ExcludedModules []string `json:"excludedModules"`
}

type upstream struct {
	Ref     string `json:"ref"`
	Version string `json:"version"`
}

// representation of an out of sync module
type oosMod struct {
	Name      string     `json:"name"`
	Version   string     `json:"version"`
	Upstreams []upstream `json:"upstreams"`
}

func main() {
	l, _ := zap.NewProduction()
	logger := l.Sugar()

	if len(os.Args) < 2 {
		fmt.Printf("USAGE: %s [PATH_TO_CONFIG_FILE]\n", os.Args[0])
		os.Exit(1)
	}

	// --- 1. parse config
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		fatal(err)
	}

	cfg := new(config)
	if err := yaml.Unmarshal(b, cfg); err != nil {
		fatal(err)
	}

	excludedMods := make(map[string]any)
	for _, mod := range cfg.ExcludedModules {
		excludedMods[mod] = nil
	}

	// --- 2. project mods
	projectModules, err := modulesFromGoModFile()
	if err != nil {
		fatal(err)
	}

	// --- 3. upstream mods
	upstreamModules, err := modulesFromUpstreamModGraph(cfg.UpstreamRefs)
	if err != nil {
		fatal(err)
	}

	oosMods := make([]oosMod, 0)

	// --- 4. validate
	// for each module in our project,
	// if it matches an upstream module,
	// then for each upstream module,
	// if project module version doesn't match upstream version,
	// then we add the version and the ref to the list of out of sync modules.
	for mod, version := range projectModules {
		if _, ok := excludedMods[mod]; ok {
			logger.Infof("skipped: %s", mod)
			continue
		}

		if versionToRef, ok := upstreamModules[mod]; ok {
			outOfSyncUpstream := make([]upstream, 0)

			for upstreamVersion, upstreamRef := range versionToRef {
				if version == upstreamVersion { // pass if version in sync.
					continue
				}

				outOfSyncUpstream = append(outOfSyncUpstream, upstream{
					Ref:     upstreamRef,
					Version: upstreamVersion,
				})
			}

			if len(outOfSyncUpstream) == 0 { // pass if no out of sync upstreams.
				continue
			}

			oosMods = append(oosMods, oosMod{
				Name:      mod,
				Version:   version,
				Upstreams: outOfSyncUpstream,
			})
		}
	}

	if len(oosMods) == 0 {
		fmt.Println("🎉 Success!")
		os.Exit(0)
	}

	b, err = json.MarshalIndent(map[string]any{"outOfSyncModules": oosMods}, "", "  ")
	if err != nil {
		fatal(err)
	}

	fmt.Println(string(b))
	os.Exit(1)
}

func modulesFromGoModFile() (map[string]string, error) {
	b, err := os.ReadFile(modFile)
	if err != nil {
		return nil, err
	}

	f, err := modfile.Parse(modFile, b, nil)
	if err != nil {
		return nil, err
	}

	out := make(map[string]string)
	for _, mod := range f.Require {
		out[mod.Mod.Path] = mod.Mod.Version
	}

	return out, nil
}

func modulesFromUpstreamModGraph(upstreamRefList []string) (map[string]map[string]string, error) {
	b, err := exec.Command("go", "mod", "graph").Output()
	if err != nil {
		return nil, err
	}

	graph := string(b)

	// upstreamRefs is a set of user specified upstream modules.
	// The set has 2 functions:
	//   1. Check if `go mod graph` modules are one of the user specified upstream modules.
	//   2. Mark if a user specified upstream module was found in the module graph.
	// If a user specified upstream module is not found, gomodcheck will exit with an error.
	upstreamRefs := make(map[string]bool)
	for _, ref := range upstreamRefList {
		upstreamRefs[ref] = false
	}

	modToVersionToUpstreamRef := make(map[string]map[string]string)
	for _, line := range strings.Split(graph, "\n") {
		ref := strings.SplitN(line, "@", 2)[0]

		if _, ok := upstreamRefs[ref]; !ok {
			continue
		}

		upstreamRefs[ref] = true // mark the ref as found

		kv := strings.SplitN(strings.SplitN(line, " ", 2)[1], "@", 2)
		name := kv[0]
		version := kv[1]

		if _, ok := modToVersionToUpstreamRef[name]; !ok {
			modToVersionToUpstreamRef[name] = make(map[string]string)
		}

		modToVersionToUpstreamRef[name][version] = ref
	}

	notFoundErr := ""
	for ref, found := range upstreamRefs {
		if !found {
			notFoundErr = fmt.Sprintf("%s%s, ", notFoundErr, ref)
		}
	}

	if notFoundErr != "" {
		return nil, fmt.Errorf("cannot verify modules: "+
			"the following specified upstream module(s) cannot be found in go.mod: [ %s ]",
			strings.TrimSuffix(notFoundErr, ", "))
	}

	return modToVersionToUpstreamRef, nil
}

func fatal(err error) {
	fmt.Printf("❌ %s\n", err.Error())
	os.Exit(1)
}
