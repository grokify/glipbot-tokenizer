package templates

import (
	"golang.org/x/oauth2"
)

type EmailData struct {
	Token *oauth2.Token
}
