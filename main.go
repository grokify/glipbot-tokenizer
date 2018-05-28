package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"

	sp "github.com/SparkPost/gosparkpost"
	"github.com/grokify/gotilla/config"
	"github.com/grokify/gotilla/net/anyhttp"
	hum "github.com/grokify/gotilla/net/httputilmore"
	uu "github.com/grokify/gotilla/net/urlutil"
	"github.com/grokify/oauth2more/sparkpost"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"github.com/grokify/glipbot-tokenizer/templates"
	ro "github.com/grokify/oauth2more/ringcentral"
)

const (
	HeaderXServerURL      = "X-Server-URL"
	RedirectUriProduction = "/oauth2callback/production"
	RedirectUriSandbox    = "/oauth2callback/sandbox"
)

type Handler struct {
	AppPort      int
	AppServerUrl string
}

func (h *Handler) handleAnyRequestHome(aRes anyhttp.Response, aReq anyhttp.Request) {
	log.WithFields(log.Fields{"handler": "handleAnyRequestHome"}).Info("StartHandler")
	aRes.SetStatusCode(http.StatusOK)
	aRes.SetContentType(hum.ContentTypeTextHtmlUtf8)
	aRes.SetBodyBytes([]byte(templates.HomePage(
		templates.HomeData{AppServerUrl: h.AppServerUrl})))
}

type UserData struct {
	AppCredentials ro.ApplicationCredentials `json:"appCreds,omitempty"`
	Token          *oauth2.Token             `json:"token,omitempty"`
}

func (h *Handler) handleAnyRequestOAuth2CallbackProd(aRes anyhttp.Response, aReq anyhttp.Request) {
	log.WithFields(log.Fields{"handler": "handleAnyRequestOAuth2CallbackProd"}).Info("StartHandler")
	aRes.SetHeader(HeaderXServerURL, ro.ServerURLProduction)
	h.handleAnyRequestOAuth2Callback(aRes, aReq)
}

func (h *Handler) handleAnyRequestOAuth2CallbackSand(aRes anyhttp.Response, aReq anyhttp.Request) {
	log.WithFields(log.Fields{"handler": "handleAnyRequestOAuth2CallbackSand"}).Info("StartHandler")
	aRes.SetHeader(HeaderXServerURL, ro.ServerURLSandbox)
	h.handleAnyRequestOAuth2Callback(aRes, aReq)
}

func getAppCredentials(aReq anyhttp.Request, rcServerUrl string) ro.ApplicationCredentials {
	appCreds := ro.ApplicationCredentials{
		ServerURL:    rcServerUrl,
		ClientID:     aReq.QueryArgs().GetString("clientId"),
		ClientSecret: aReq.QueryArgs().GetString("clientSecret")}
	if rcServerUrl == ro.ServerURLProduction {
		appCreds.RedirectURL = uu.JoinAbsolute(os.Getenv("APP_SERVER_URL"), RedirectUriProduction)
	} else {
		appCreds.RedirectURL = uu.JoinAbsolute(os.Getenv("APP_SERVER_URL"), RedirectUriSandbox)
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
		log.WithFields(log.Fields{"Error:": err.Error()}).Info("ERR_PARSE_FORM")
	}

	authCode := aReq.QueryArgs().GetString("code")
	log.WithFields(log.Fields{"authCodeReceived": authCode}).Info("authCodeReceived")

	// Exchange auth code for token
	appCredentials := getAppCredentials(aReq, string(aRes.GetHeader(HeaderXServerURL)))

	log.WithFields(log.Fields{"authCode": authCode}).Info("authCode")
	log.WithFields(log.Fields{"clientId": appCredentials.ClientID}).Info("clientId")
	log.WithFields(log.Fields{"email": aReq.QueryArgs().GetString("email")}).Info("email")
	log.WithFields(log.Fields{"redirectUrl": appCredentials.RedirectURL}).Info("redirectUrl")

	log.WithFields(log.Fields{"requestUrl": string(aReq.RequestURI())}).Info("requestUrl")

	o2Config := appCredentials.Config()
	token, err := o2Config.Exchange(oauth2.NoContext, authCode)
	if err != nil {
		log.WithFields(log.Fields{
			"oauth2": "tokenExchangeError",
		}).Info(err.Error())
		return
	}

	log.WithFields(log.Fields{"tokenReceived": token.AccessToken}).Info("tokenReceived")

	sendTokenEmail(token, aReq.QueryArgs().GetString("email"))

	aRes.SetStatusCode(http.StatusOK)
}

func sendTokenEmail(token *oauth2.Token, recipient string) {
	client, err := sparkpost.NewApiClient(os.Getenv("SPARKPOST_API_KEY"))
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error()}).Warn("Email")
	}

	// Create a Transmission using an inline Recipient List
	// and inline email Content.
	emailData := templates.EmailData{Token: token}
	tx := &sp.Transmission{
		Recipients: []string{recipient},
		Content: sp.Content{
			HTML:    templates.TokenEmail(emailData),
			From:    os.Getenv("SPARKPOST_EMAIL_SENDER"),
			Subject: "Your Glip Bot Token is here."}}

	id, _, err := client.Send(tx)
	if err != nil {
		log.Fatal(err)
	}
	log.WithFields(log.Fields{"email-id": id}).Info("email")
}

func serveNetHttp(h Handler) {
	log.Info("STARTING_NET_HTTP")
	mux := http.NewServeMux()

	mux.HandleFunc("/oauth2callback/production", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.handleAnyRequestOAuth2CallbackProd(anyhttp.NewResReqNetHttp(w, r))
	}))
	mux.HandleFunc("/oauth2callback/production/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.handleAnyRequestOAuth2CallbackProd(anyhttp.NewResReqNetHttp(w, r))
	}))
	mux.HandleFunc("/oauth2callback/sandbox", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.handleAnyRequestOAuth2CallbackSand(anyhttp.NewResReqNetHttp(w, r))
	}))
	mux.HandleFunc("/oauth2callback/sandbox/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.handleAnyRequestOAuth2CallbackSand(anyhttp.NewResReqNetHttp(w, r))
	}))
	mux.HandleFunc("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.handleAnyRequestHome(anyhttp.NewResReqNetHttp(w, r))
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
		AppServerUrl: os.Getenv("APP_SERVER_URL")}

	serveNetHttp(handler)
}
