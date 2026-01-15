// Package control is the main package for the control service.
package control

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aide-family/magicbox/hello"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"github.com/aide-family/goddess/cmd"
	configv1 "github.com/aide-family/goddess/pkg/config/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "control",
		Short: "goddess control service",
		Long:  "goddess control service for managing gateway configurations",
		Annotations: map[string]string{
			"group": cmd.ServiceCommands,
		},
		Run: run,
	}
	flags.addFlags(cmd)
	return cmd
}

type GatewayConfig struct {
	Config          *configv1.Gateway              `json:"config"`
	Version         string                         `json:"version"`
	PriorityConfigs map[string]*PriorityConfigData `json:"priorityConfigs"`
}

type PriorityConfigData struct {
	Config  *configv1.PriorityConfig `json:"config"`
	Version string                   `json:"version"`
}

type GatewayFeatures struct {
	Features map[string]bool `json:"features"`
}

type ControlService struct {
	dataDir  string
	mu       sync.RWMutex
	configs  map[string]*GatewayConfig   // key: gateway name
	features map[string]*GatewayFeatures // key: gateway name
}

func NewControlService(dataDir string) (*ControlService, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	return &ControlService{
		dataDir:  dataDir,
		configs:  make(map[string]*GatewayConfig),
		features: make(map[string]*GatewayFeatures),
	}, nil
}

func (s *ControlService) GetGatewayRelease(ctx context.Context, gateway, ipAddr, lastVersion string, lastPriorityVersions map[string]string) (*LoadResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cfg, exists := s.configs[gateway]
	if !exists {
		// Try to load from file
		if err := s.loadConfigFromFile(gateway); err != nil {
			return nil, err
		}
		cfg = s.configs[gateway]
		if cfg == nil {
			return nil, newStatusError(http.StatusNotFound, "gateway config not found")
		}
	}

	// Check if version changed
	if lastVersion != "" && cfg.Version == lastVersion {
		// Check priority configs version
		priorityChanged := false
		if len(lastPriorityVersions) > 0 {
			for key, version := range lastPriorityVersions {
				if pc, ok := cfg.PriorityConfigs[key]; !ok || pc.Version != version {
					priorityChanged = true
					break
				}
			}
		} else if len(cfg.PriorityConfigs) > 0 {
			priorityChanged = true
		}

		if !priorityChanged {
			return nil, newStatusError(http.StatusNotModified, "config not modified")
		}
	}

	// Convert config to JSON
	configJSON, err := protojson.Marshal(cfg.Config)
	if err != nil {
		return nil, err
	}

	// Build priority configs
	priorityConfigs := make([]*PriorityConfigItem, 0, len(cfg.PriorityConfigs))
	for key, pc := range cfg.PriorityConfigs {
		pcJSON, err := protojson.Marshal(pc.Config)
		if err != nil {
			log.Warnf("Failed to marshal priority config %s: %v", key, err)
			continue
		}
		priorityConfigs = append(priorityConfigs, &PriorityConfigItem{
			Key:     key,
			Config:  string(pcJSON),
			Version: pc.Version,
		})
	}

	return &LoadResponse{
		Config:          string(configJSON),
		Version:         cfg.Version,
		PriorityConfigs: priorityConfigs,
	}, nil
}

func (s *ControlService) GetGatewayFeatures(ctx context.Context, gateway, ipAddr string) (*LoadFeatureResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	features, exists := s.features[gateway]
	if !exists {
		// Try to load from file
		if err := s.loadFeaturesFromFile(gateway); err != nil {
			return nil, err
		}
		features = s.features[gateway]
		if features == nil {
			// Return default features
			return &LoadFeatureResponse{
				Gateway:  gateway,
				Features: make(map[string]bool),
			}, nil
		}
	}

	return &LoadFeatureResponse{
		Gateway:  gateway,
		Features: features.Features,
	}, nil
}

func (s *ControlService) loadConfigFromFile(gateway string) error {
	configPath := filepath.Join(s.dataDir, gateway, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		return err
	}

	cfg := &configv1.Gateway{}
	if err := protojson.Unmarshal(jsonData, cfg); err != nil {
		return err
	}

	// Load version
	versionPath := filepath.Join(s.dataDir, gateway, "version.txt")
	version := "v1.0.0"
	if versionData, err := os.ReadFile(versionPath); err == nil {
		version = strings.TrimSpace(string(versionData))
		if version == "" {
			version = "v1.0.0"
		}
	}

	// Load priority configs
	priorityConfigs := make(map[string]*PriorityConfigData)
	priorityDir := filepath.Join(s.dataDir, gateway, "priority")
	if entries, err := os.ReadDir(priorityDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
				continue
			}
			key := entry.Name()[:len(entry.Name())-5] // remove .yaml
			pcPath := filepath.Join(priorityDir, entry.Name())
			pcData, err := os.ReadFile(pcPath)
			if err != nil {
				continue
			}
			pcJSON, err := yaml.YAMLToJSON(pcData)
			if err != nil {
				continue
			}
			pc := &configv1.PriorityConfig{}
			if err := protojson.Unmarshal(pcJSON, pc); err != nil {
				continue
			}
			versionPath := filepath.Join(priorityDir, key+".version.txt")
			pcVersion := "v1.0.0"
			if vData, err := os.ReadFile(versionPath); err == nil {
				pcVersion = strings.TrimSpace(string(vData))
				if pcVersion == "" {
					pcVersion = "v1.0.0"
				}
			}
			priorityConfigs[key] = &PriorityConfigData{
				Config:  pc,
				Version: pcVersion,
			}
		}
	}

	s.mu.Lock()
	s.configs[gateway] = &GatewayConfig{
		Config:          cfg,
		Version:         version,
		PriorityConfigs: priorityConfigs,
	}
	s.mu.Unlock()

	return nil
}

func (s *ControlService) loadFeaturesFromFile(gateway string) error {
	featuresPath := filepath.Join(s.dataDir, gateway, "features.json")
	if _, err := os.Stat(featuresPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(featuresPath)
	if err != nil {
		return err
	}

	features := &GatewayFeatures{}
	if err := json.Unmarshal(data, features); err != nil {
		return err
	}

	s.mu.Lock()
	s.features[gateway] = features
	s.mu.Unlock()

	return nil
}

type LoadResponse struct {
	Config          string                `json:"config"`
	Version         string                `json:"version"`
	PriorityConfigs []*PriorityConfigItem `json:"priorityConfigs"`
}

type PriorityConfigItem struct {
	Key     string `json:"key"`
	Config  string `json:"config"`
	Version string `json:"version"`
}

type LoadFeatureResponse struct {
	Gateway  string          `json:"gateway"`
	Features map[string]bool `json:"features"`
}

type statusError struct {
	statusCode int
	message    string
}

func (e *statusError) Error() string {
	return e.message
}

func newStatusError(code int, msg string) error {
	return &statusError{statusCode: code, message: msg}
}

func run(_ *cobra.Command, _ []string) {
	ctx := context.Background()

	// Create control service
	service, err := NewControlService(flags.dataDir)
	if err != nil {
		log.Fatalf("failed to create control service: %v", err)
	}

	// Create HTTP server
	httpSrv := kratoshttp.NewServer(
		kratoshttp.Address(flags.httpAddr),
		kratoshttp.Middleware(
			recovery.Recovery(),
		),
	)

	// Register handlers
	httpSrv.HandleFunc("/v1/control/gateway/release", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		gateway := r.URL.Query().Get("gateway")
		ipAddr := r.URL.Query().Get("ip_addr")
		lastVersion := r.URL.Query().Get("last_version")

		// Parse lastPriorityVersions
		lastPriorityVersions := make(map[string]string)
		if r.URL.Query().Get("supportPriorityConfig") == "1" {
			versions := r.URL.Query()["lastPriorityVersions"]
			for _, v := range versions {
				parts := strings.SplitN(v, "=", 2)
				if len(parts) == 2 {
					lastPriorityVersions[parts[0]] = parts[1]
				}
			}
		}

		resp, err := service.GetGatewayRelease(r.Context(), gateway, ipAddr, lastVersion, lastPriorityVersions)
		if err != nil {
			if se, ok := err.(*statusError); ok {
				w.WriteHeader(se.statusCode)
				if se.statusCode != http.StatusNotModified {
					json.NewEncoder(w).Encode(map[string]string{"error": se.message})
				}
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	httpSrv.HandleFunc("/v1/control/gateway/features", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		gateway := r.URL.Query().Get("gateway")
		ipAddr := r.URL.Query().Get("ip_addr")

		resp, err := service.GetGatewayFeatures(r.Context(), gateway, ipAddr)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	app := kratos.New(
		kratos.Name("control"),
		kratos.Context(ctx),
		kratos.Server(httpSrv),
	)

	globalFlags := cmd.GetGlobalFlags()
	envOpts := []hello.Option{
		hello.WithVersion(globalFlags.Version),
		hello.WithID(globalFlags.Hostname),
		hello.WithEnv("PROD"),
		hello.WithMetadata(map[string]string{}),
		hello.WithName(globalFlags.Name),
	}
	hello.SetEnvWithOption(envOpts...)
	hello.Hello()

	log.Infof("control service listening on %s", flags.httpAddr)
	if err := app.Run(); err != nil {
		log.Errorf("failed to run control service: %v", err)
	}
}
