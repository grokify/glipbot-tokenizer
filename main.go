package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/grokify/gostor"
	"github.com/grokify/gostor/redis"
	"github.com/grokify/gotilla/config"
	"github.com/grokify/gotilla/crypto/hash/argon2"
	ju "github.com/grokify/gotilla/encoding/jsonutil"
	"github.com/grokify/gotilla/net/anyhttp"
	hum "github.com/grokify/gotilla/net/httputilmore"
	uu "github.com/grokify/gotilla/net/urlutil"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"github.com/grokify/glipbot-tokenizer/templates"
	ro "github.com/grokify/oauth2more/ringcentral"
)

const (
	CookieNameSession     = "gbtsession"
	CookieSalt            = "Trying to get my Glip bot token the easy way!"
	HeaderXServerURL      = "X-Server-URL"
	RedirectUriProduction = "/oauth2callback/production"
	RedirectUriSandbox    = "/oauth2callback/sandbox"
)

type Handler struct {
	AppPort int
	Cache   gostor.Client
}

func createUserCookie(aReq anyhttp.Request) *anyhttp.Cookie {
	return &anyhttp.Cookie{
		Name:  CookieNameSession,
		Value: buildCacheKey(aReq)}
}

func buildCacheKey(aReq anyhttp.Request) string {
	return argon2.HashSimpleBase32(
		[]byte(buildCacheKeyRaw(aReq)),
		[]byte(CookieSalt),
		true)
}

func buildCacheKeyRaw(aReq anyhttp.Request) string {
	return strings.Join([]string{aReq.RemoteAddress(), string(aReq.UserAgent())}, ",")
}

func (h *Handler) handleAnyRequestHome(aRes anyhttp.Response, aReq anyhttp.Request) {
	aRes.SetCookie(createUserCookie(aReq))
	aRes.SetStatusCode(http.StatusOK)
	aRes.SetContentType(hum.ContentTypeTextHtmlUtf8)
	aRes.SetBodyBytes([]byte(templates.HomePage()))
}

type UserData struct {
	AppCredentials ro.ApplicationCredentials `json:"appCreds,omitempty"`
	Token          *oauth2.Token             `json:"token,omitempty"`
}

func (h *Handler) handleAnyRequestButton(aRes anyhttp.Response, aReq anyhttp.Request) {
	log.Info("ANY_REQ_PROC1_S1")
	err := aReq.ParseForm()
	if err != nil {
		log.Fatal(err)
	}
	log.Info("ANY_REQ_PROC1_S2")
	params := []string{"serverUrl", "appId", "clientId", "clientSecret"}
	for _, param := range params {
		param := strings.TrimSpace(aReq.PostArgs().GetString(param))
		if len(param) == 0 {
			aRes.SetStatusCode(http.StatusTemporaryRedirect)
			aRes.SetHeader(hum.HeaderLocation, "/")
			return
		}
	}
	log.Info("ANY_REQ_PROC1_S3")
	cacheKey := buildCacheKey(aReq)
	log.WithFields(log.Fields{
		"cacheKey": cacheKey,
	}).Info("PROC1")
	userData := UserData{
		AppCredentials: ro.ApplicationCredentials{
			ServerURL:     aReq.PostArgs().GetString("serverUrl"),
			ApplicationID: aReq.PostArgs().GetString("appId"),
			ClientID:      aReq.PostArgs().GetString("clientId"),
			ClientSecret:  aReq.PostArgs().GetString("clientSecret")}}

	if err = h.setUserData(cacheKey, userData); err != nil {
		log.Fatal(err)
	}

	tmplData := templates.ButtonData{
		ApplicationID: userData.AppCredentials.ApplicationID}

	aRes.SetStatusCode(http.StatusOK)
	aRes.SetContentType(hum.ContentTypeTextHtmlUtf8)
	aRes.SetBodyBytes([]byte(templates.ButtonPage(tmplData)))
}

func (h *Handler) getUserData(key string) (UserData, error) {
	userData := UserData{}
	val := h.Cache.GetOrEmptyString(key)
	if len(strings.TrimSpace(val)) == 0 {
		return userData, fmt.Errorf("No value for [%v]", key)
	}
	err := json.Unmarshal([]byte(val), &userData)
	return userData, err
}

func (h *Handler) setUserData(cacheKey string, userData UserData) error {
	if bytes, err := json.Marshal(userData); err != nil {
		return err
	} else {
		h.Cache.SetString(cacheKey, string(bytes))
		return nil
	}
}

func (h *Handler) handleAnyRequestOAuth2CallbackProd(aRes anyhttp.Response, aReq anyhttp.Request) {
	aRes.SetHeader(HeaderXServerURL, ro.ServerURLProduction)
	h.handleAnyRequestOAuth2Callback(aRes, aReq)
}

func (h *Handler) handleAnyRequestOAuth2CallbackSand(aRes anyhttp.Response, aReq anyhttp.Request) {
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
	v.Add("eamil", aReq.QueryArgs().GetString("eamil"))
	appCreds.RedirectURL += "?" + v.Encode()
	return appCreds
}

func (h *Handler) handleAnyRequestOAuth2Callback(aRes anyhttp.Response, aReq anyhttp.Request) {
	err := aReq.ParseForm()
	if err != nil {
		log.WithFields(log.Fields{"Error:": err.Error()}).Info("ERR_PARSE_FORM")
	}
	cacheKey := buildCacheKey(aReq)
	userData, err := h.getUserData(cacheKey)
	if err != nil {
		log.WithFields(log.Fields{
			"cache": fmt.Sprintf("Cannot retrieve Cache for [%v] [%v]",
				cacheKey,
				ju.MustMarshalString(userData, true)),
		}).Info(cacheKey)
	}
	log.WithFields(log.Fields{"cacheKey": cacheKey}).Info("cacheKey")

	authCode := aReq.QueryArgs().GetString("code")
	log.WithFields(log.Fields{"authCodeReceived": authCode}).Info("authCodeReceived")

	// Exchange auth code for token
	userData.AppCredentials = getAppCredentials(aReq, string(aRes.GetHeader(HeaderXServerURL)))

	log.WithFields(log.Fields{"clientId": userData.AppCredentials.ClientID}).Info("clientId")
	log.WithFields(log.Fields{"clientSecret": userData.AppCredentials.ClientSecret}).Info("clientSecret")
	log.WithFields(log.Fields{"email": aReq.QueryArgs().GetString("email")}).Info("email")

	o2Config := userData.AppCredentials.Config()
	token, err := o2Config.Exchange(oauth2.NoContext, authCode)
	if err != nil {
		log.WithFields(log.Fields{
			"oauth2": "tokenExchangeError",
		}).Info(err.Error())
		return
	}

	userData.Token = token
	if err = h.setUserData(cacheKey, userData); err != nil {
		log.Fatal(err)
	}
	log.WithFields(log.Fields{"tokenReceived": token.AccessToken}).Info("tokenReceived")

	fmt.Printf("SET TOKEN FOR [%v] [%v]", cacheKey, token.AccessToken)
	aRes.SetStatusCode(http.StatusOK)
}

func (h *Handler) handleAnyRequestInstalled(aRes anyhttp.Response, aReq anyhttp.Request) {
	cacheKey := buildCacheKey(aReq)
	userData, err := h.getUserData(cacheKey)
	if err != nil {
		log.WithFields(log.Fields{
			"cache": fmt.Sprintf("Cannot retrieve Cache for [%v] [%v]",
				cacheKey,
				ju.MustMarshalString(userData, true)),
		}).Info(cacheKey)
	}

	if err = h.setUserData(cacheKey, userData); err != nil {
		log.Fatal(err)
	}

	data := templates.InstalledData{
		Token: userData.Token,
	}

	fmt.Printf("SET TOKEN FOR [%v] [%v]", cacheKey, data.Token.AccessToken)
	aRes.SetStatusCode(http.StatusOK)
	aRes.SetContentType(hum.ContentTypeTextHtmlUtf8)
	aRes.SetBodyBytes([]byte(templates.InstalledPage(data)))
}

func serveNetHttp(h Handler) {
	log.Info("STARTING_NET_HTTP")
	mux := http.NewServeMux()
	mux.HandleFunc("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.handleAnyRequestHome(anyhttp.NewResReqNetHttp(w, r))
	}))
	mux.HandleFunc("/button", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.handleAnyRequestButton(anyhttp.NewResReqNetHttp(w, r))
	}))
	mux.HandleFunc("/button/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.handleAnyRequestButton(anyhttp.NewResReqNetHttp(w, r))
	}))
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
	mux.HandleFunc("/installed", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.handleAnyRequestInstalled(anyhttp.NewResReqNetHttp(w, r))
	}))
	mux.HandleFunc("/installed/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.handleAnyRequestInstalled(anyhttp.NewResReqNetHttp(w, r))
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
		AppPort: port,
		Cache: redis.NewClient(gostor.Config{
			Host:        "127.0.0.1",
			Port:        6379,
			Password:    "",
			CustomIndex: 0})}

	engine := strings.ToLower(strings.TrimSpace(os.Getenv("HTTP_ENGINE")))
	if len(engine) == 0 {
		engine = "nethttp"
	}

	switch engine {
	case "fasthttp":
		serveNetHttp(handler)
	default:
		serveNetHttp(handler)
	}
}
