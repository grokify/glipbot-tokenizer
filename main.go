package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	stdlog "log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	sp "github.com/SparkPost/gosparkpost"
	"github.com/grokify/goauth"
	"github.com/grokify/goauth/sparkpost"
	"github.com/grokify/mogo/config"
	"github.com/grokify/mogo/net/http/httputilmore"
	"github.com/grokify/mogo/net/urlutil"
	"github.com/grokify/sogo/net/http/anyhttp"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"

	"github.com/grokify/glipbot-tokenizer/templates"
	ro "github.com/grokify/goauth/ringcentral"
)

const (
	HeaderXServerURL      = "X-Server-URL"
	RedirectURLProduction = "/oauth2callback/production"
	RedirectURLSandbox    = "/oauth2callback/sandbox"
)

type Handler struct {
	AppPort      int
	AppServerURL string
}

func (h *Handler) handleAnyRequestHome(aRes anyhttp.Response, aReq anyhttp.Request) {
	log.Info().
		Str("handler", "handleAnyRequestHome").
		Str("reqURL", string(aReq.RequestURI())).
		Msg("StartHandler")
	aRes.SetStatusCode(http.StatusOK)
	aRes.SetContentType(httputilmore.ContentTypeTextHTMLUtf8)
	_, err := aRes.SetBodyBytes([]byte(templates.HomePage(
		templates.HomeData{AppServerURL: h.AppServerURL})))
	if err != nil {
		log.Warn().
			Err(err).
			Msg("failure on `anyhttp.Response.SetBodyBytes`")
	}
}

type UserData struct {
	AppCredentials goauth.CredentialsOAuth2 `json:"appCreds,omitempty"`
	Token          *oauth2.Token            `json:"token,omitempty"`
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

func getAppCredentials(aReq anyhttp.Request, rcServerURL string) goauth.CredentialsOAuth2 {
	appCreds := goauth.CredentialsOAuth2{
		ServerURL:    rcServerURL,
		ClientID:     aReq.QueryArgs().GetString("clientId"),
		ClientSecret: aReq.QueryArgs().GetString("clientSecret")}
	if rcServerURL == ro.ServerURLProduction {
		appCreds.RedirectURL = urlutil.JoinAbsolute(os.Getenv("APP_SERVER_URL"), RedirectURLProduction)
	} else {
		appCreds.RedirectURL = urlutil.JoinAbsolute(os.Getenv("APP_SERVER_URL"), RedirectURLSandbox)
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
	token, err := o2Config.Exchange(context.Background(), authCode)
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

func serveNetHTTP(h Handler) {
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

	svr := httputilmore.NewServerTimeouts(fmt.Sprintf(":%v", h.AppPort), mux, 1*time.Second)
	stdlog.Fatal(svr.ListenAndServe())
	/*
	   done := make(chan bool)
	   go http.ListenAndServe(fmt.Sprintf(":%v", h.AppPort), mux)
	   log.Printf("Server listening on port %v", h.AppPort)
	   <-done
	*/
}

func main() {
	_, err := config.LoadDotEnv([]string{os.Getenv("ENV_PATH"), "./.env"}, 1)
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

	serveNetHTTP(handler)
}
