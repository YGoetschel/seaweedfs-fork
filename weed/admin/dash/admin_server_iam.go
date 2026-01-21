package dash

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/seaweedfs/seaweedfs/weed/glog"
	"github.com/seaweedfs/seaweedfs/weed/iam/integration"
	"github.com/seaweedfs/seaweedfs/weed/iam/ldap"
	"github.com/seaweedfs/seaweedfs/weed/iam/policy"
)

// RoleMappingRule defines a rule to map external groups to IAM roles
type RoleMappingRule struct {
	Value string `json:"value"` // Group name (e.g. "developers")
	Role  string `json:"role"`  // IAM Role name (e.g. "ReadOnly")
}

// RoleMappingConfig holds role mapping rules
type RoleMappingConfig struct {
	Rules []RoleMappingRule `json:"rules"`
}

type ProvidersConfig struct {
	STS struct {
		Providers []struct {
			Name        string                 `json:"name"`
			Type        string                 `json:"type"`
			Config      map[string]interface{} `json:"config"`
			RoleMapping RoleMappingConfig      `json:"roleMapping"`
		} `json:"providers"`
	} `json:"sts"`
}

// cachedRoleMappings stores loaded mappings
var cachedRoleMappings map[string]string

// initIAMManager initializes the IAM manager
func (s *AdminServer) initIAMManager() {
	s.iamManager = integration.NewIAMManager()
	cachedRoleMappings = make(map[string]string)

	// Initial configuration
	iamConfig := &integration.IAMConfig{
		STS: &integration.STSConfig{
			TokenDuration:    "24h",
			MaxSessionLength: "24h",
			Issuer:           "seaweedfs-admin",
			// SigningKey will be loaded or generated/persisted
		},
		Policy: &policy.PolicyEngineConfig{
			DefaultEffect: "Deny", // Secure default
		},
		Roles: &integration.RoleStoreConfig{
			StoreType: "cached-filer",
		},
	}

// Try to load custom configuration from Filer
	var configData []byte
	var filerAddress string

	// Wait for Filer discovery (up to 30 seconds)
	for i := 0; i < 30; i++ {
		filerAddress = s.GetFilerAddress()
		if filerAddress != "" {
			break
		}
		if i%5 == 0 {
			glog.V(0).Infof("Waiting for Filer discovery to load IAM config...")
		}
		time.Sleep(1 * time.Second)
	}

	if filerAddress != "" {
		url := fmt.Sprintf("http://%s/etc/iam/iam_config.json", filerAddress)
		glog.V(0).Infof("Loading IAM configuration from %s", url)
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			if data, err := io.ReadAll(resp.Body); err == nil {
				configData = data
				glog.V(0).Infof("Successfully loaded IAM config of size: %d bytes", len(configData))
				if err := json.Unmarshal(data, iamConfig); err != nil {
					glog.Errorf("Failed to parse IAM config into IAMConfig struct: %v", err)
				}
			}
		} else {
			// If config doesn't exist (404), it's fine, we will create it
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				glog.V(0).Infof("IAM configuration not found at %s, will create new one", url)
			} else {
				glog.Errorf("Failed to load IAM config from Filer at %s: err=%v, status=%s", url, err, resp.Status)
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
	} else {
		glog.Errorf("Failed to load IAM config: No Filer discovered after 30 seconds")
	}

	// Ensure we have a signing key (persist it if generated)
	if len(iamConfig.STS.SigningKey) == 0 {
		glog.V(0).Infof("No signing key found in configuration. Generating a new persistent key.")
		
		// Generate 32 bytes
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			glog.Errorf("Failed to generate random signing key: %v", err)
			// Fallback to ephemeral insecure key if generation fails (should happen rarely)
			iamConfig.STS.SigningKey = []byte(randomString(32))
		} else {
			iamConfig.STS.SigningKey = key
		}
		
		// Persist the configuration if we have a filer
		if filerAddress != "" {
			url := fmt.Sprintf("http://%s/etc/iam/iam_config.json", filerAddress)
			
			// Marshal with indentation for readability
			data, err := json.MarshalIndent(iamConfig, "", "    ")
			if err != nil {
				glog.Errorf("Failed to marshal IAM config for persistence: %v", err)
			} else {
				req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(data))
				if err != nil {
					glog.Errorf("Failed to create request to persist IAM config: %v", err)
				} else {
					req.Header.Set("Content-Type", "application/json")
					client := &http.Client{Timeout: 5 * time.Second}
					resp, err := client.Do(req)
					if err != nil {
						glog.Errorf("Failed to persist IAM config to %s: %v", url, err)
					} else {
						defer resp.Body.Close()
						if resp.StatusCode >= 200 && resp.StatusCode < 300 {
							glog.V(0).Infof("Successfully persisted IAM configuration with new signing key")
						} else {
							body, _ := io.ReadAll(resp.Body)
							glog.Errorf("Failed to persist IAM config, status: %s, body: %s", resp.Status, string(body))
						}
					}
				}
			}
		}
	}


	// Clear providers from config to prevent auto-loading failure
	// We will manually load and register them below
	if iamConfig.STS != nil {
		// Providers not in integration config, no need to clear
	}

	// Initialize with filer address provider
	err := s.iamManager.Initialize(iamConfig, func() string {
		return s.GetFilerAddress()
	}, nil)
	if err != nil {
		glog.Errorf("Failed to initialize IAM manager: %v", err)
		return
	}

	// Load providers and role mappings
	if len(configData) > 0 {
		var provConfig ProvidersConfig
		if err := json.Unmarshal(configData, &provConfig); err == nil {
			for _, p := range provConfig.STS.Providers {
				// Cache role mappings
				for _, rule := range p.RoleMapping.Rules {
					cachedRoleMappings[rule.Value] = rule.Role
				}

				if p.Type == "ldap" {
					ldapProvider := ldap.NewLDAPProvider(p.Name)
					if err := ldapProvider.Initialize(p.Config); err != nil {
						glog.Errorf("Failed to initialize LDAP provider %s: %v", p.Name, err)
						continue
					}
					if err := s.iamManager.RegisterIdentityProvider(ldapProvider); err != nil {
						glog.Errorf("Failed to register LDAP provider: %v", err)
					} else {
						glog.V(0).Infof("Registered LDAP provider: %s", p.Name)
					}
				}
			}
		}
	}
}

// ResolveRolesFromGroups maps LDAP groups to SeaweedFS IAM Roles
func (s *AdminServer) ResolveRolesFromGroups(groups []string) []string {
	var roles []string
	
	// Check cached mappings
	for _, group := range groups {
		if role, ok := cachedRoleMappings[group]; ok {
			// Avoid duplicates
			found := false
			for _, r := range roles {
				if r == role {
					found = true
					break
				}
			}
			if !found {
				roles = append(roles, role)
			}
		}
	}
	
	return roles
}

func randomString(n int) string {
    bytes := make([]byte, n)
    if _, err := rand.Read(bytes); err != nil {
        return "fallback_random_string_" + time.Now().String()
    }
    return hex.EncodeToString(bytes)
}
