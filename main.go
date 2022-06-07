package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"

	sp "github.com/SparkPost/gosparkpost"
	"github.com/grokify/goauth/credentials"
	"github.com/grokify/goauth/sparkpost"
	"github.com/grokify/gohttp/anyhttp"
	"github.com/grokify/mogo/config"
	"github.com/grokify/mogo/net/httputilmore"
	"github.com/grokify/mogo/net/urlutil"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"

	"github.com/grokify/glipbot-tokenizer/templates"
	ro "github.com/grokify/goauth/ringcentral"
)

const (
	HeaderXServerURL      = "X-Server-URL"
	RedirectUriProduction = "/oauth2callback/production"
	RedirectUriSandbox    = "/oauth2callback/sandbox"
)

type Handler struct {
	AppPort      int
	AppServerURL string
}

func (h *Handler) handleAnyRequestHome(aRes anyhttp.Response, aReq anyhttp.Request) {
	log.Info().
		Str("handler", "handleAnyRequestHome").
		Msg("StartHandler")
	aRes.SetStatusCode(http.StatusOK)
	aRes.SetContentType(httputilmore.ContentTypeTextHTMLUtf8)
	aRes.SetBodyBytes([]byte(templates.HomePage(
		templates.HomeData{AppServerURL: h.AppServerURL})))
}

type UserData struct {
	AppCredentials credentials.CredentialsOAuth2 `json:"appCreds,omitempty"`
	Token          *oauth2.Token                 `json:"token,omitempty"`
}

func (h *Handler) handleAnyRequestOAuth2CallbackProd(aRes anyhttp.Response, aReq anyhttp.Request) {
	log.Info().
		Str("handler", "handleAnyRequestOAuth2CallbackProd").
		Msg("StartHandler")
	aRes.SetHeader(HeaderXServerURL, ro.ServerURLProduction)
	h.handleAnyRequestOAuth2Callback(aRes, aReq)
}

func (h *Handler) handleAnyRequestOAuth2CallbackSand(aRes anyhttp.Response, aReq anyhttp.Request) {
	log.Info().
		Str("handler", "handleAnyRequestOAuth2CallbackSand").
		Msg("StartHandler")
	aRes.SetHeader(HeaderXServerURL, ro.ServerURLSandbox)
	h.handleAnyRequestOAuth2Callback(aRes, aReq)
}

func getAppCredentials(aReq anyhttp.Request, rcServiceURL string) credentials.CredentialsOAuth2 {
	appCreds := credentials.CredentialsOAuth2{
		ServiceURL:   rcServiceURL,
		ClientID:     aReq.QueryArgs().GetString("clientId"),
		ClientSecret: aReq.QueryArgs().GetString("clientSecret")}
	if rcServiceURL == ro.ServerURLProduction {
		appCreds.RedirectURL = urlutil.JoinAbsolute(os.Getenv("APP_SERVER_URL"), RedirectUriProduction)
	} else {
		appCreds.RedirectURL = urlutil.JoinAbsolute(os.Getenv("APP_SERVER_URL"), RedirectUriSandbox)
	}
	v := url.Values{}
	v.Add("clientId", appCreds.ClientID)
	v.Add("clientSecret", appCreds.ClientSecret)
	v.Add("email", aReq.QueryArgs().GetString("email"))
	appCreds.RedirectURL += "?" + v.Encode()
	return appCreds
}

func (h *Handler) handleAnyRequestOAuth2Callback(aRes anyhttp.Response, aReq anyhttp.Request) {
	err := aReq.ParseForm()
	if err != nil {
		log.Warn().
			Err(err).
			Msg("ERR_PARSE_FORM")
		aRes.SetStatusCode(http.StatusBadRequest)
		return
	}

	authCode := aReq.QueryArgs().GetString("code")
	log.Info().
		Str("authCodeReceived", authCode).
		Msg("authCodeReceived")

	// Exchange auth code for token
	appCredentials := getAppCredentials(aReq, string(aRes.GetHeader(HeaderXServerURL)))

	log.Info().
		Str("authCode", authCode).
		Str("clientId", appCredentials.ClientID).
		Str("email", aReq.QueryArgs().GetString("email")).
		Str("redirectUrl", appCredentials.RedirectURL).
		Str("requestUrl", string(aReq.RequestURI())).
		Msg("authCode")

	o2Config := appCredentials.Config()
	token, err := o2Config.Exchange(oauth2.NoContext, authCode)
	if err != nil {
		log.Warn().
			Err(err).
			Msg("OAuth2CodeToTokenExchangeError")
		aRes.SetStatusCode(http.StatusBadRequest)
		return
	}

	log.Info().
		Str("token", token.AccessToken).
		Msg("tokenReceived")

	sendTokenEmail(token, aReq.QueryArgs().GetString("email"))

	aRes.SetStatusCode(http.StatusOK)
}

func sendTokenEmail(token *oauth2.Token, recipient string) {
	client, err := sparkpost.NewAPIClient(os.Getenv("SPARKPOST_API_KEY"))
	if err != nil {
		log.Warn().
			Err(err).
			Str("stage", "get email client").
			Msg("email")
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		log.Warn().
			Err(err).
			Str("stage", "marshal token").
			Msg("email")
	}
	attach := sp.Attachment{
		MIMEType: httputilmore.ContentTypeTextPlainUtf8,
		Filename: "token.json.txt",
		B64Data:  base64.StdEncoding.EncodeToString(data)}

	// Create a Transmission using an inline Recipient List
	// and inline email Content.
	emailData := templates.EmailData{Token: token}
	tx := &sp.Transmission{
		Recipients: []string{recipient},
		Content: sp.Content{
			HTML:        templates.TokenEmail(emailData),
			From:        os.Getenv("SPARKPOST_EMAIL_SENDER"),
			Subject:     "Your Glip Bot Token is here.",
			Attachments: []sp.Attachment{attach}}}

	id, _, err := client.Send(tx)
	if err != nil {
		log.Fatal().Err(err).Msg("client.Send")
	}
	log.Info().
		Str("email-id", id).
		Msg("email")
}

func serveNetHttp(h Handler) {
	log.Info().Msg("STARTING_NET_HTTP")
	mux := http.NewServeMux()

	mux.HandleFunc("/oauth2callback/production", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.handleAnyRequestOAuth2CallbackProd(anyhttp.NewResReqNetHTTP(w, r))
	}))
	mux.HandleFunc("/oauth2callback/production/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.handleAnyRequestOAuth2CallbackProd(anyhttp.NewResReqNetHTTP(w, r))
	}))
	mux.HandleFunc("/oauth2callback/sandbox", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.handleAnyRequestOAuth2CallbackSand(anyhttp.NewResReqNetHTTP(w, r))
	}))
	mux.HandleFunc("/oauth2callback/sandbox/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.handleAnyRequestOAuth2CallbackSand(anyhttp.NewResReqNetHTTP(w, r))
	}))
	mux.HandleFunc("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.handleAnyRequestHome(anyhttp.NewResReqNetHTTP(w, r))
	}))

	done := make(chan bool)
	go http.ListenAndServe(fmt.Sprintf(":%v", h.AppPort), mux)
	log.Printf("Server listening on port %v", h.AppPort)
	<-done
}

func main() {
	err := config.LoadDotEnvSkipEmpty(os.Getenv("ENV_PATH"), "./.env")
	if err != nil {
		panic(err)
	}

	// PORT environment variable is automatically set for Heroku.
	portRaw := os.Getenv("PORT")
	port, err := strconv.Atoi(portRaw)
	if err != nil {
		port = 3000
	}

	handler := Handler{
		AppPort:      port,
		AppServerURL: os.Getenv("APP_SERVER_URL")}

	serveNetHttp(handler)
}
