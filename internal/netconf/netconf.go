// Package netconf drives the wireless adapter from the kiosk, so a device
// locked to one app can still be put on a venue network without unlocking it.
package netconf

// Network is one wireless network the adapter can see.
type Network struct {
	SSID   string `json:"ssid"`
	Signal int    `json:"signal"` // percent
	Secure bool   `json:"secure"`
	Saved  bool   `json:"saved"`
}

type Status struct {
	Connected bool   `json:"connected"`
	SSID      string `json:"ssid,omitempty"`
	Signal    int    `json:"signal,omitempty"`
}
