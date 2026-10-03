//go:build windows

package netconf

import (
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const openProfileTemplate = `<?xml version="1.0"?>
<WLANProfile xmlns="http://www.microsoft.com/networking/WLAN/profile/v1">
  <name>{{SSID}}</name>
  <SSIDConfig><SSID><hex>{{HEX}}</hex><name>{{SSID}}</name></SSID></SSIDConfig>
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
  <SSIDConfig><SSID><hex>{{HEX}}</hex><name>{{SSID}}</name></SSID></SSIDConfig>
  <connectionType>ESS</connectionType>
  <connectionMode>auto</connectionMode>
  <MSM><security>
    <authEncryption>
      <authentication>{{AUTH}}</authentication>
      <encryption>{{CIPHER}}</encryption>
      <useOneX>false</useOneX>
    </authEncryption>
    <sharedKey>
      <keyType>passPhrase</keyType>
      <protected>false</protected>
      <keyMaterial>{{KEY}}</keyMaterial>
    </sharedKey>
  </security></MSM>
</WLANProfile>`

// profileSecurity turns what a scan reports into what a profile names. A
// WPA2 profile on a WPA3-only network never joins, which is how a phone
// hotspot turns away a correct password.
func profileSecurity(auth, cipher string) (string, string, error) {
	enc := "AES"
	if strings.EqualFold(cipher, "TKIP") {
		enc = "TKIP"
	}
	switch strings.ToLower(auth) {
	case "wpa3-personal":
		return "WPA3SAE", "AES", nil
	case "wpa-personal":
		return "WPAPSK", enc, nil
	case "wpa2-personal", "":
		return "WPA2PSK", enc, nil
	}
	if strings.Contains(strings.ToLower(auth), "enterprise") {
		return "", "", fmt.Errorf("this network wants a username as well as a password, which the stand cannot send. Use another network")
	}
	return "WPA2PSK", enc, nil
}

func tempProfile(ssid, password, auth, cipher string) (string, error) {
	template := profileTemplate
	if password == "" {
		template = openProfileTemplate
	}
	authName, enc, err := profileSecurity(auth, cipher)
	if password != "" && err != nil {
		return "", err
	}
	xmlDoc := strings.ReplaceAll(template, "{{SSID}}", escape(ssid))
	// Windows matches the network on the hex bytes, so a name with an
	// apostrophe or an emoji in it still finds its network.
	xmlDoc = strings.ReplaceAll(xmlDoc, "{{HEX}}", strings.ToUpper(hex.EncodeToString([]byte(ssid))))
	xmlDoc = strings.ReplaceAll(xmlDoc, "{{KEY}}", escape(password))
	xmlDoc = strings.ReplaceAll(xmlDoc, "{{AUTH}}", authName)
	xmlDoc = strings.ReplaceAll(xmlDoc, "{{CIPHER}}", enc)

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
