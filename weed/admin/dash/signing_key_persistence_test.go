package dash

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/seaweedfs/seaweedfs/weed/iam/integration"
	"github.com/stretchr/testify/assert"
)

// TestSigningKeyPersistence verifies that initIAMManager generates and persists a signing key
// when one is missing from the configuration.
// CAUTION: This test interacts with the local Filer at localhost:8888.
// It backs up and restores /etc/iam/iam_config.json.
func TestSigningKeyPersistence(t *testing.T) {
	filerAddress := "localhost:8888"
	configFile := "/etc/iam/iam_config.json"
	configURL := fmt.Sprintf("http://%s%s", filerAddress, configFile)

	// 1. Backup existing config
	var backupData []byte
	resp, err := http.Get(configURL)
	if err == nil && resp.StatusCode == http.StatusOK {
		backupData, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("Backed up existing config (%d bytes)\n", len(backupData))
	} else if resp != nil {
		resp.Body.Close()
	}

	// Helper to restore backup
	defer func() {
		if len(backupData) > 0 {
			req, _ := http.NewRequest(http.MethodPut, configURL, bytes.NewBuffer(backupData))
			req.Header.Set("Content-Type", "application/json")
			http.DefaultClient.Do(req)
			fmt.Println("Restored original config backup")
		}
	}()

	// 2. Delete existing config to simulate fresh start
	req, _ := http.NewRequest(http.MethodDelete, configURL, nil)
	http.DefaultClient.Do(req)

	// 3. Setup AdminServer with mocked Filer address
	server := &AdminServer{
		cachedFilers:         []string{filerAddress},
		lastFilerUpdate:      time.Now(),
		filerCacheExpiration: time.Hour,
	}

	// 4. Run initialization (First Run - Should Generate Key)
	server.initIAMManager()

	// 4. Run initialization (First Run - Should Generate Key)
	server.initIAMManager()

	// 5. Verify Persistence to Filer
	resp, err = http.Get(configURL)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	savedData, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var savedConfig1 integration.IAMConfig
	json.Unmarshal(savedData, &savedConfig1)
	
	firstKey := savedConfig1.STS.SigningKey
	assert.NotEmpty(t, firstKey, "Signing key should be generated and persisted")
	assert.Equal(t, 32, len(firstKey), "Signing key should be 32 bytes")
	fmt.Printf("First run generated key: %x\n", firstKey)

	// 6. Run initialization AGAIN (Second Run - Should Load Existing Key)
	// Clear manager to ensure reload (although initIAMManager creates new one)
	server.iamManager = nil
	server.initIAMManager()

	// Read config again to ensure it wasn't overwritten with a new random key
	resp, err = http.Get(configURL)
	assert.NoError(t, err)
	
	savedData2, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	
	var savedConfig2 integration.IAMConfig
	json.Unmarshal(savedData2, &savedConfig2)
	secondKey := savedConfig2.STS.SigningKey
	
	// Verify keys match (Persistence worked!)
	assert.Equal(t, firstKey, secondKey, "Subsequent initialization should load/keep the persisted key")
}
