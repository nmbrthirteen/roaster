//go:build windows

package netconf

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
)

const openProfileTemplate = `<?xml version="1.0"?>
<WLANProfile xmlns="http://www.microsoft.com/networking/WLAN/profile/v1">
  <name>{{SSID}}</name>
  <SSIDConfig><SSID><name>{{SSID}}</name></SSID></SSIDConfig>
  <connectionType>ESS</connectionType>
  <connectionMode>auto</connectionMode>
  <MSM><security>
    <authEncryption>
      <authentication>open</authentication>
      <encryption>none</encryption>
      <useOneX>false</useOneX>
    </authEncryption>
  </security></MSM>
</WLANProfile>`

const profileTemplate = `<?xml version="1.0"?>
<WLANProfile xmlns="http://www.microsoft.com/networking/WLAN/profile/v1">
  <name>{{SSID}}</name>
  <SSIDConfig><SSID><name>{{SSID}}</name></SSID></SSIDConfig>
  <connectionType>ESS</connectionType>
  <connectionMode>auto</connectionMode>
  <MSM><security>
    <authEncryption>
      <authentication>WPA2PSK</authentication>
      <encryption>AES</encryption>
      <useOneX>false</useOneX>
    </authEncryption>
    <sharedKey>
      <keyType>passPhrase</keyType>
      <protected>false</protected>
      <keyMaterial>{{KEY}}</keyMaterial>
    </sharedKey>
  </security></MSM>
</WLANProfile>`

func tempProfile(ssid, password string) (string, error) {
	template := profileTemplate
	if password == "" {
		template = openProfileTemplate
	}
	xmlDoc := strings.ReplaceAll(template, "{{SSID}}", escape(ssid))
	xmlDoc = strings.ReplaceAll(xmlDoc, "{{KEY}}", escape(password))

	path := filepath.Join(os.TempDir(), "roaster-wlan.xml")
	// The file holds the passphrase in clear text, so it is readable only by
	// this account and deleted as soon as netsh has read it.
	if err := os.WriteFile(path, []byte(xmlDoc), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func remove(path string) { _ = os.Remove(path) }

func escape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
