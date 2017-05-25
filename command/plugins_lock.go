package command

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

func (m *Meta) providerPluginsLock() *pluginSHA256LockFile {
	return &pluginSHA256LockFile{
		Filename: filepath.Join(m.pluginDir(), "providers.json"),
	}
}

type pluginSHA256LockFile struct {
	Filename string
}

// Read loads the lock information from the file and returns it. If the file
// cannot be read, an empty map is returned to indicate that _no_ providers
// are acceptable, since the user must run "terraform init" to lock some
// providers before a context can be created.
func (pf *pluginSHA256LockFile) Read() map[string][]byte {
	// Returning an empty map is different than nil because it causes
	// us to reject all plugins as uninitialized, rather than applying no
	// constraints at all.
	//
	// We don't surface any specific errors here because we want it to all
	// roll up into our more-user-friendly error that appears when plugin
	// constraint verification fails during context creation.
	digests := make(map[string][]byte)

	buf, err := ioutil.ReadFile(pf.Filename)
	if err != nil {
		// This is expected if the user runs any context-using command before
		// running "terraform init".
		log.Printf("[INFO] Failed to read plugin lock file %s: %s", pf.Filename, err)
		return digests
	}

	var strDigests map[string]string
	err = json.Unmarshal(buf, &strDigests)
	if err != nil {
		// This should never happen unless the user directly edits the file.
		log.Printf("[WARNING] Plugin lock file %s failed to parse as JSON: %s", pf.Filename, err)
		return digests
	}

	for name, strDigest := range strDigests {
		var digest []byte
		_, err := fmt.Sscanf(strDigest, "%x", &digest)
		if err == nil {
			digests[name] = digest
		} else {
			// This should never happen unless the user directly edits the file.
			log.Printf("[WARNING] Plugin lock file %s has invalid digest for %q", pf.Filename, name)
		}
	}

	return digests
}

// Write persists lock information to disk, where it will be retrieved by
// future calls to Read. This entirely replaces any previous lock information,
// so the given map must be comprehensive.
func (pf *pluginSHA256LockFile) Write(digests map[string][]byte) error {
	strDigests := map[string]string{}
	for name, digest := range digests {
		strDigests[name] = fmt.Sprintf("%x", digest)
	}

	buf, err := json.MarshalIndent(strDigests, "", "  ")
	if err != nil {
		// should never happen
		return fmt.Errorf("failed to serialize plugin lock as JSON: %s", err)
	}

	os.MkdirAll(
		filepath.Dir(pf.Filename), os.ModePerm,
	) // ignore error since WriteFile below will generate a better one anyway

	return ioutil.WriteFile(pf.Filename, buf, os.ModePerm)
}