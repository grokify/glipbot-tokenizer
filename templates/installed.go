package templates

import (
	"encoding/json"

	"golang.org/x/oauth2"
)

type InstalledData struct {
	Token *oauth2.Token
}

func (d *InstalledData) TokenBytes() []byte {
	if bytes, err := json.MarshalIndent(d.Token, "", "  "); err != nil {
		return []byte("")
	} else {
		return bytes
	}
}
