package filer_etc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/seaweedfs/seaweedfs/weed/credential"
	"github.com/seaweedfs/seaweedfs/weed/filer"
	"github.com/seaweedfs/seaweedfs/weed/glog"
	"github.com/seaweedfs/seaweedfs/weed/pb/filer_pb"
	"github.com/seaweedfs/seaweedfs/weed/pb/iam_pb"
	"strings"
)

func (store *FilerEtcStore) LoadConfiguration(ctx context.Context) (*iam_pb.S3ApiConfiguration, error) {
	s3cfg := &iam_pb.S3ApiConfiguration{}

	err := store.withFilerClient(func(client filer_pb.SeaweedFilerClient) error {
		// Step 1: Load all users from /etc/iam/users/ directory (primary source)
		entries, err := filer.ListEntry(nil, client, filer.IamUsersDirectory, "", 1000, "")
		
		if err == nil && len(entries) > 0 {
			glog.V(1).Infof("Loading IAM users from %s (individual files)", filer.IamUsersDirectory)
			for _, entry := range entries {
				if entry.IsDirectory {
					continue
				}
				if !strings.HasSuffix(entry.Name, ".json") {
					continue
				}
				content, err := filer.ReadInsideFiler(client, filer.IamUsersDirectory, entry.Name)
				if err != nil {
					glog.Warningf("Failed to read user file %s/%s: %v", filer.IamUsersDirectory, entry.Name, err)
					continue
				}
				
				identity := &iam_pb.Identity{}
				if err := filer.ParseS3ConfigurationFromBytes(content, identity); err != nil {
					glog.Warningf("Failed to parse user file %s/%s: %v", filer.IamUsersDirectory, entry.Name, err)
					continue
				}
				
				s3cfg.Identities = append(s3cfg.Identities, identity)
			}
		}
		
		// Step 2: Always try to load iam_config.json for global config (STS, etc.) and fallback users
		// We use HTTP hack because ReadInsideFiler fails on chunked files, similar to migration logic
		
		// Get filer address from store functions (protected by mutex in helper, but we need raw access or use the helper?)
		// store.withFilerClient gives us a client, but doesn't give us the HTTP address directly?
		// Actually, we can't easily access the address here because it's hidden in the store closure context?
		// Wait, FilerEtcStore struct has 'filerAddressFunc'.
		
		// But we are inside 'withFilerClient'. We should be able to just access store fields if we hold the lock?
		// No, `withFilerClient` holds RLock. We are inside the callback.
		// Wait, `withFilerClient` calls `fn`. It holds RLock *before* calling `fn`?
		// Let's check `filer_etc_store.go`.
		// It holds lock to get address, then unlocks, then calls `pb.WithGrpcFilerClient`.
		// So we are NOT holding the lock here. Safe to verify.
		
		store.mu.RLock()
		filerAddress := ""
		if store.filerAddressFunc != nil {
			filerAddress = string(store.filerAddressFunc())
		}
		store.mu.RUnlock()
		
		if filerAddress != "" {
			fileUrl := fmt.Sprintf("http://%s%s/%s", filerAddress, filer.IamConfigDirectory, filer.IamIdentityFile)
			resp, err := http.Get(fileUrl)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == 200 {
					content, err := io.ReadAll(resp.Body)
					if err == nil && len(content) > 0 {
						tempCfg := &iam_pb.S3ApiConfiguration{}
						if err := filer.ParseS3ConfigurationFromBytes(content, tempCfg); err == nil {
							// Merge global config (Accounts, etc. - STS is usually here if proto supports it)
							s3cfg.Accounts = tempCfg.Accounts
							// Merge other fields if necessary
							
							// Only append users if we didn't find any individual files
							if len(s3cfg.Identities) == 0 {
								s3cfg.Identities = tempCfg.Identities
							}
						}
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		return s3cfg, err
	}

	glog.V(1).Infof("Successfully loaded IAM configuration with %d identities", len(s3cfg.Identities))
	
	// Log loaded identities for debugging
	if glog.V(2) {
		for _, identity := range s3cfg.Identities {
			credCount := len(identity.Credentials)
			actionCount := len(identity.Actions)
			glog.V(2).Infof("  Identity: %s (credentials: %d, actions: %d)",
				identity.Name, credCount, actionCount)
			for _, cred := range identity.Credentials {
				glog.V(3).Infof("    Access Key: %s", cred.AccessKey)
			}
		}
	}

	return s3cfg, nil
}

func (store *FilerEtcStore) SaveConfiguration(ctx context.Context, config *iam_pb.S3ApiConfiguration) error {
	return store.withFilerClient(func(client filer_pb.SeaweedFilerClient) error {
		var buf bytes.Buffer
		if err := filer.ProtoToText(&buf, config); err != nil {
			return fmt.Errorf("failed to marshal configuration: %w", err)
		}
		return filer.SaveInsideFiler(client, filer.IamConfigDirectory, filer.IamIdentityFile, buf.Bytes())
	})
}

func (store *FilerEtcStore) CreateUser(ctx context.Context, identity *iam_pb.Identity) error {
	// Check if user already exists
	existing, err := store.GetUser(ctx, identity.Name)
	if err == nil && existing != nil {
		return credential.ErrUserAlreadyExists
	}

	// Write user to individual file
	return store.withFilerClient(func(client filer_pb.SeaweedFilerClient) error {
		userFile := fmt.Sprintf("%s.json", identity.Name)
		var buf bytes.Buffer
		if err := filer.ProtoToText(&buf, identity); err != nil {
			return fmt.Errorf("failed to serialize user %s: %w", identity.Name, err)
		}
		return filer.SaveInsideFiler(client, filer.IamUsersDirectory, userFile, buf.Bytes())
	})

}

func (store *FilerEtcStore) GetUser(ctx context.Context, username string) (*iam_pb.Identity, error) {
	config, err := store.LoadConfiguration(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	for _, identity := range config.Identities {
		if identity.Name == username {
			return identity, nil
		}
	}

	return nil, credential.ErrUserNotFound
}

func (store *FilerEtcStore) UpdateUser(ctx context.Context, username string, identity *iam_pb.Identity) error {
	// Check if user exists
	_, err := store.GetUser(ctx, username)
	if err != nil {
		return credential.ErrUserNotFound
	}

	// Write updated user to individual file
	return store.withFilerClient(func(client filer_pb.SeaweedFilerClient) error {
		userFile := fmt.Sprintf("%s.json", username)
		var buf bytes.Buffer
		if err := filer.ProtoToText(&buf, identity); err != nil {
			return fmt.Errorf("failed to serialize user %s: %w", username, err)
		}
		return filer.SaveInsideFiler(client, filer.IamUsersDirectory, userFile, buf.Bytes())
	})
}

func (store *FilerEtcStore) DeleteUser(ctx context.Context, username string) error {
	// Delete individual user file
	return store.withFilerClient(func(client filer_pb.SeaweedFilerClient) error {
		userFile := fmt.Sprintf("%s.json", username)
		err := filer.DeleteInsideFiler(client, filer.IamUsersDirectory, userFile)
		if err == filer_pb.ErrNotFound {
			return credential.ErrUserNotFound
		}
		return err
	})
}

func (store *FilerEtcStore) ListUsers(ctx context.Context) ([]string, error) {
	config, err := store.LoadConfiguration(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	var usernames []string
	for _, identity := range config.Identities {
		usernames = append(usernames, identity.Name)
	}

	return usernames, nil
}

func (store *FilerEtcStore) GetUserByAccessKey(ctx context.Context, accessKey string) (*iam_pb.Identity, error) {
	config, err := store.LoadConfiguration(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	for _, identity := range config.Identities {
		for _, credential := range identity.Credentials {
			if credential.AccessKey == accessKey {
				return identity, nil
			}
		}
	}

	return nil, credential.ErrAccessKeyNotFound
}

func (store *FilerEtcStore) CreateAccessKey(ctx context.Context, username string, cred *iam_pb.Credential) error {
	// Get the user
	user, err := store.GetUser(ctx, username)
	if err != nil {
		return err
	}

	// Check if access key already exists
	for _, existingCred := range user.Credentials {
		if existingCred.AccessKey == cred.AccessKey {
			return fmt.Errorf("access key %s already exists", cred.AccessKey)
		}
	}

	// Add new credential
	user.Credentials = append(user.Credentials, cred)

	// Write updated user back to individual file
	return store.withFilerClient(func(client filer_pb.SeaweedFilerClient) error {
		userFile := fmt.Sprintf("%s.json", username)
		var buf bytes.Buffer
		if err := filer.ProtoToText(&buf, user); err != nil {
			return fmt.Errorf("failed to serialize user %s: %w", username, err)
		}
		return filer.SaveInsideFiler(client, filer.IamUsersDirectory, userFile, buf.Bytes())
	})
}

func (store *FilerEtcStore) DeleteAccessKey(ctx context.Context, username string, accessKey string) error {
	// Get the user
	user, err := store.GetUser(ctx, username)
	if err != nil {
		return err
	}

	// Find and remove the credential
	found := false
	for i, cred := range user.Credentials {
		if cred.AccessKey == accessKey {
			user.Credentials = append(user.Credentials[:i], user.Credentials[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return credential.ErrAccessKeyNotFound
	}

	// Write updated user back to individual file
	return store.withFilerClient(func(client filer_pb.SeaweedFilerClient) error {
		userFile := fmt.Sprintf("%s.json", username)
		var buf bytes.Buffer
		if err := filer.ProtoToText(&buf, user); err != nil {
			return fmt.Errorf("failed to serialize user %s: %w", username, err)
		}
		return filer.SaveInsideFiler(client, filer.IamUsersDirectory, userFile, buf.Bytes())
	})
}
