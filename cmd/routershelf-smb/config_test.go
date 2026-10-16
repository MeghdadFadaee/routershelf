package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (config, string) {
	t.Helper()
	mount, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(mount, "private-storage")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "smb.json")
	// Public synthetic credential fixture; never deploy it.
	c := config{Mount: mount, Root: root, User: "storage", Hash: "9b675d1cbdc67f1620c6442c7b07bc63", Allow: []string{"127.0.0.1/32"}}
	return c, p
}
func saveConfig(t *testing.T, c config, p string) {
	t.Helper()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
}
func TestConfigurationAndIndependence(t *testing.T) {
	c, p := fixture(t)
	// Omit fields to exercise secure defaults, even with conflicting web settings.
	b := `{"storage_mount":` + quote(c.Mount) + `,"root":` + quote(c.Root) + `,"user":"storage","nt_hash":` + quote(c.Hash) + `,"allow":["127.0.0.1/32"]}`
	if err := os.WriteFile(p, []byte(b), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RS_DOMAIN", "invalid.example")
	t.Setenv("RS_ROOT", "/nonexistent/web")
	t.Setenv("RS_USB_MOUNT", "/missing")
	t.Setenv("RS_SMB_CONFIG", "/missing")
	got, check, err := loadConfig([]string{"-config", p, "-check-config"})
	if err != nil {
		t.Fatal(err)
	}
	if !check || !got.ReadOnly || got.Listen != "127.0.0.1:1445" || got.Root != c.Root || got.UID != 65533 || got.GID != 65533 {
		t.Fatalf("defaults/precedence: %+v", got)
	}
	t.Setenv("RS_SMB_CONFIG", p)
	if _, _, err := loadConfig([]string{"-check-config"}); err != nil {
		t.Fatal(err)
	}
}
func quote(s string) string { b, _ := json.Marshal(s); return string(b) }
func TestUnsafeConfiguration(t *testing.T) {
	tests := map[string]func(*config, string){
		"protected root":     func(c *config, _ string) { c.ProtectedPaths = []string{c.Root} },
		"system mount":       func(c *config, _ string) { c.Mount = "/" },
		"uid overflow":       func(c *config, _ string) { value := int64(2147483648); c.UID = int(value) },
		"control name":       func(c *config, _ string) { c.User = "bad\x01user" },
		"wildcard":           func(c *config, _ string) { c.Listen = "0.0.0.0:445" },
		"public":             func(c *config, _ string) { c.Listen = "203.0.113.1:445" },
		"named port":         func(c *config, _ string) { c.Listen = "127.0.0.1:http" },
		"all peers":          func(c *config, _ string) { c.Allow = []string{"0.0.0.0/0"} },
		"cross boundary":     func(c *config, _ string) { c.Allow = []string{"172.0.0.0/8"} },
		"no peers":           func(c *config, _ string) { c.Allow = nil },
		"root account":       func(c *config, _ string) { c.UID = 0 },
		"placeholder":        func(c *config, _ string) { c.Hash = strings.Repeat("0", 32) },
		"empty password":     func(c *config, _ string) { c.Hash = "31d6cfe0d16ae931b73c59d7e0c089c0" },
		"mount root":         func(c *config, _ string) { c.Root = c.Mount },
		"missing storage":    func(c *config, _ string) { c.Root = filepath.Join(c.Mount, "missing") },
		"credentials shared": func(c *config, p string) { c.Root = filepath.Dir(p); c.Mount = filepath.Dir(c.Root) },
		"public overlap":     func(c *config, _ string) { c.PublicRoots = []string{c.Root} },
		"public parent":      func(c *config, _ string) { c.PublicRoots = []string{c.Mount} },
		"bad share":          func(c *config, _ string) { c.Share = "IPC$" },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			c, p := fixture(t)
			c.Listen = "127.0.0.1:1445"
			c.Share = "Storage"
			c.UID = 65534
			c.GID = 65534
			change(&c, p)
			saveConfig(t, c, p)
			if _, _, err := loadConfig([]string{"-config", p}); err == nil {
				t.Fatal("accepted unsafe config")
			}
		})
	}
}
func TestSecretFileAndOverlap(t *testing.T) {
	c, p := fixture(t)
	c.Listen = "127.0.0.1:1445"
	c.Share = "Storage"
	c.UID = 65534
	c.GID = 65534
	c.PublicRoots = []string{c.Root}
	c.AllowPublicOverlap = true
	saveConfig(t, c, p)
	if _, _, err := loadConfig([]string{"-config", p}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadConfig([]string{"-config", p}); err == nil {
		t.Fatal("accepted readable credentials")
	}
	if err := os.Chmod(p, 0600); err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{`{"password":"do-not-log-this"}`, `{} {}`, strings.Repeat(" ", 32769)} {
		if err := os.WriteFile(p, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		_, _, err := loadConfig([]string{"-config", p})
		if err == nil || strings.Contains(err.Error(), "do-not-log-this") {
			t.Fatal("invalid config accepted or secret leaked")
		}
	}
}

func TestExplicitDiskRootView(t *testing.T) {
	c, p := fixture(t)
	c.Listen = "127.0.0.1:1445"
	c.Share = "Storage"
	c.UID = 65533
	c.GID = 65533
	c.Root = c.Mount
	c.AllowStorageRoot = true
	saveConfig(t, c, p)
	if _, _, err := loadConfig([]string{"-config", p}); err == nil {
		t.Fatal("root view must explicitly allow public overlap too")
	}
	c.AllowPublicOverlap = true
	saveConfig(t, c, p)
	got, _, err := loadConfig([]string{"-config", p})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.protectedRelative) < 2 {
		t.Fatal("disk root lacks mandatory service exclusions")
	}
	c.Root = filepath.Join(c.Mount, ".smb-service")
	if err := os.Mkdir(c.Root, 0700); err != nil {
		t.Fatal(err)
	}
	saveConfig(t, c, p)
	if _, _, err := loadConfig([]string{"-config", p}); err == nil {
		t.Fatal("protected service directory became a share")
	}
}
