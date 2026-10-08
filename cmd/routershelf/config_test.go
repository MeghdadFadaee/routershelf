package main

import "testing"

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"RS_USB_MOUNT", "RS_ROOT", "RS_CERT_CACHE", "RS_PUBLIC_SUBDIR", "RS_SERVICE_SUBDIR", "RS_DROP_PRIVILEGES", "RS_LAN_IP", "RS_HTTPS_PORT", "RS_HTTP_PORT", "RS_DOMAIN"} {
		t.Setenv(key, "")
	}
}
func TestEnvConfigurationAndOverrides(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("RS_USB_MOUNT", "/media/example")
	t.Setenv("RS_DOMAIN", "files.example.com")
	t.Setenv("RS_LAN_IP", "192.0.2.1")
	t.Setenv("RS_HTTPS_PORT", "10443")
	t.Setenv("RS_HTTP_PORT", "10080")
	t.Setenv("RS_DROP_PRIVILEGES", "true")
	c, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.root != "/media/example/public-storage" || c.certCache != "/media/example/.public-share-service/certificates" || c.listen != "192.0.2.1:10443" || !c.drop {
		t.Fatalf("incorrect derived configuration: %+v", c)
	}
	c, err = parseConfig([]string{"-listen", "127.0.0.1:9443", "-domain", "other.example.com"})
	if err != nil || c.listen != "127.0.0.1:9443" || c.domain != "other.example.com" {
		t.Fatalf("flags did not override env: %+v %v", c, err)
	}
}
func TestRejectUnsafeConfiguration(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"RS_HTTPS_PORT", "0"}, {"RS_DROP_PRIVILEGES", "perhaps"}, {"RS_DOMAIN", "https://files.example.com"}, {"RS_CERT_CACHE", "/media/example/public-storage/keys"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			clearConfigEnv(t)
			t.Setenv("RS_USB_MOUNT", "/media/example")
			t.Setenv("RS_DOMAIN", "files.example.com")
			t.Setenv(tc.key, tc.value)
			if _, err := parseConfig(nil); err == nil {
				t.Fatal("unsafe config accepted")
			}
		})
	}
}
