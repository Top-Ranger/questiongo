// SPDX-License-Identifier: Apache-2.0
// Copyright 2020,2021,2022 Marcus Soll
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	  http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	auth "github.com/Top-Ranger/auth/data"
	"github.com/Top-Ranger/questiongo/helper"
	"github.com/Top-Ranger/questiongo/registry"
	"github.com/Top-Ranger/questiongo/translation"
)

var serverMutex sync.Mutex
var serverStarted bool
var server http.Server
var rootPath string

var resultsTemplate *template.Template
var resultsAccessTemplate *template.Template
var reloadTemplate *template.Template

var dsgvo []byte
var impressum []byte

var questionnairesLock sync.RWMutex
var questionnaires map[string]Questionnaire

//go:embed static font js css
var cachedFiles embed.FS
var cssTemplates *template.Template

var robottxt = []byte(`User-agent: *
Disallow: /`)

func init() {
	var err error

	resultsAccessTemplate, err = template.ParseFS(templateFiles, "template/resultsAccess.html")
	if err != nil {
		panic(err)
	}

	resultsTemplate, err = template.New("results").Funcs(evenOddFuncMap).ParseFS(templateFiles, "template/results.html")
	if err != nil {
		panic(err)
	}

	cssTemplates, err = template.ParseFS(cachedFiles, "css/*")
	if err != nil {
		panic(err)
	}

	reloadTemplate, err = template.ParseFS(templateFiles, "template/reload.html")
	if err != nil {
		panic(err)
	}
}

type resultsTemplateStruct struct {
	Results     []template.HTML
	Key         string
	Auth        string
	Translation translation.Translation
	ServerPath  string
}

type resultsAccessTemplateStruct struct {
	Translation translation.Translation
	ServerPath  string
}

func initialiseServer() error {
	if serverStarted {
		return nil
	}
	server = http.Server{Addr: config.Address}

	// Do setup
	rootPath = strings.Join([]string{config.ServerPath, "/"}, "")

	// DSGVO
	b, err := os.ReadFile(config.PathDSGVO)
	if err != nil {
		return err
	}
	f, ok := registry.GetFormatType(config.FormatDSGVO)
	if !ok {
		return fmt.Errorf("unknown format type %s (DSGVO)", config.FormatDSGVO)
	}
	text := textTemplateStruct{f.Format(b), translation.GetDefaultTranslation(), config.ServerPath}
	output := bytes.NewBuffer(make([]byte, 0, len(text.Text)*2))
	textTemplate.Execute(output, text)
	dsgvo = output.Bytes()

	http.HandleFunc(strings.Join([]string{config.ServerPath, "/dsgvo.html"}, ""), func(rw http.ResponseWriter, r *http.Request) {
		rw.Write(dsgvo)
	})

	// Impressum
	b, err = os.ReadFile(config.PathImpressum)
	if err != nil {
		return err
	}
	f, ok = registry.GetFormatType(config.FormatImpressum)
	if !ok {
		return fmt.Errorf("unknown format type %s (impressum)", config.FormatImpressum)
	}
	text = textTemplateStruct{f.Format(b), translation.GetDefaultTranslation(), config.ServerPath}
	text.Text = template.HTML(strings.Join([]string{string(text.Text), "<p><img style=\"max-width: min(500px, 80%);\" src=\"/static/Logo.svg\" alt=\"Logo\"></p>"}, ""))
	output = bytes.NewBuffer(make([]byte, 0, len(text.Text)*2))
	textTemplate.Execute(output, text)
	impressum = output.Bytes()
	http.HandleFunc(strings.Join([]string{config.ServerPath, "/impressum.html"}, ""), func(rw http.ResponseWriter, r *http.Request) {
		rw.Write(impressum)
	})

	etag := fmt.Sprint("\"", strconv.FormatInt(time.Now().Unix(), 10), "\"")
	etagCompare := strings.TrimSuffix(etag, "\"")
	etagCompareApache := strings.Join([]string{etagCompare, "-"}, "")       // Dirty hack for apache2, who appends -gzip inside the quotes if the file is compressed, thus preventing If-None-Match matching the ETag
	etagCompareCaddy := strings.Join([]string{"W/", etagCompare, "\""}, "") // Dirty hack for caddy, who appends W/ before the quotes if the file is compressed, thus preventing If-None-Match matching the ETag

	staticHandle := func(rw http.ResponseWriter, r *http.Request) {
		// Check for ETag
		v, ok := r.Header["If-None-Match"]
		if ok {
			for i := range v {
				if v[i] == etag || v[i] == etagCompareCaddy || strings.HasPrefix(v[i], etagCompareApache) {
					rw.WriteHeader(http.StatusNotModified)
					return
				}
			}
		}

		// Send file if existing in cache
		path := r.URL.Path
		path = strings.TrimPrefix(path, config.ServerPath)
		path = strings.TrimPrefix(path, "/")

		if strings.HasPrefix(path, "css/") {
			// special case
			path = strings.TrimPrefix(path, "css/")
			rw.Header().Set("ETag", etag)
			rw.Header().Set("Cache-Control", "public, max-age=43200")
			rw.Header().Set("Content-Type", "text/css")
			err := cssTemplates.ExecuteTemplate(rw, path, struct{ ServerPath string }{config.ServerPath})
			if err != nil {
				rw.WriteHeader(http.StatusNotFound)
				log.Println("server:", err)
			}
			return
		}

		data, err := cachedFiles.Open(path)
		if err != nil {
			rw.WriteHeader(http.StatusNotFound)
		} else {
			rw.Header().Set("ETag", etag)
			rw.Header().Set("Cache-Control", "public, max-age=43200")
			switch {
			case strings.HasSuffix(path, ".svg"):
				rw.Header().Set("Content-Type", "image/svg+xml")
			case strings.HasSuffix(path, ".ttf"):
				rw.Header().Set("Content-Type", "application/x-font-truetype")
			case strings.HasSuffix(path, ".js"):
				rw.Header().Set("Content-Type", "application/javascript")
			default:
				rw.Header().Set("Content-Type", "text/plain")
			}
			io.Copy(rw, data)
		}
	}

	http.HandleFunc(strings.Join([]string{config.ServerPath, "/css/"}, ""), staticHandle)
	http.HandleFunc(strings.Join([]string{config.ServerPath, "/static/"}, ""), staticHandle)
	http.HandleFunc(strings.Join([]string{config.ServerPath, "/font/"}, ""), staticHandle)
	http.HandleFunc(strings.Join([]string{config.ServerPath, "/js/"}, ""), staticHandle)

	// robots.txt
	http.HandleFunc(strings.Join([]string{config.ServerPath, "/robots.txt"}, ""), func(rw http.ResponseWriter, r *http.Request) {
		rw.Write(robottxt)
	})

	// Questionnaires
	questionnairesLock.Lock()
	questionnaires, err = LoadAllQuestionnaires(config.DataFolder)
	questionnairesLock.Unlock()
	if err != nil {
		return err
	}
	http.HandleFunc(strings.Join([]string{config.ServerPath, "/answer.html"}, ""), answerHandle)
	http.HandleFunc(strings.Join([]string{config.ServerPath, "/results.html"}, ""), resultsHandle)
	http.HandleFunc(strings.Join([]string{config.ServerPath, "/reload.html"}, ""), reloadHandle)
	http.HandleFunc(strings.Join([]string{config.ServerPath, "/results.zip"}, ""), func(w http.ResponseWriter, r *http.Request) { resultDownloadHandle(w, r, "zip") })
	http.HandleFunc(strings.Join([]string{config.ServerPath, "/results.csv"}, ""), func(w http.ResponseWriter, r *http.Request) { resultDownloadHandle(w, r, "csv") })
	http.HandleFunc("/", questionnaireHandle)

	return nil
}

func questionnaireHandle(rw http.ResponseWriter, r *http.Request) {
	if r.URL.Path == rootPath || r.URL.Path == config.ServerPath || r.URL.Path == "/" {
		t := errorTemplateStruct{"<h1>QuestionGo!</h1>", translation.GetDefaultTranslation(), config.ServerPath}
		errorTemplate.Execute(rw, t)
		return
	}

	rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	key := r.URL.Path
	if !strings.HasPrefix(key, config.ServerPath) {
		rw.WriteHeader(http.StatusNotFound)
		translationStruct := translation.GetDefaultTranslation()
		t := errorTemplateStruct{template.HTML(fmt.Sprintf("<h1>%s</h1>", translationStruct.CanNotFindQuestionnaire)), translationStruct, config.ServerPath}
		errorTemplate.Execute(rw, t)
		return
	}
	key = strings.TrimPrefix(key, config.ServerPath)
	key = strings.TrimLeft(key, "/")
	questionnairesLock.RLock()
	q, ok := questionnaires[key]
	questionnairesLock.RUnlock()
	if !ok {
		rw.WriteHeader(http.StatusNotFound)
		translationStruct := translation.GetDefaultTranslation()
		t := errorTemplateStruct{template.HTML(fmt.Sprintf("<h1>%s</h1>", translationStruct.CanNotFindQuestionnaire)), translationStruct, config.ServerPath}
		errorTemplate.Execute(rw, t)
		return
	}

	if !q.Open {
		translationStruct, err := translation.GetTranslation(q.Language)
		if err != nil {
			log.Printf("server: error while getting translation (%s) for questionnaire %s: %s", q.Language, key, err.Error())
			translationStruct = translation.GetDefaultTranslation()
		}

		t := errorTemplateStruct{helper.SanitiseString(fmt.Sprintf(translationStruct.QuestionnaireClosed, q.Contact)), translationStruct, config.ServerPath}
		errorTemplate.Execute(rw, t)
		return
	}
	query := r.URL.Query()
	_, main := query["main"]
	_, end := query["end"]

	if main {
		q.WriteQuestions(rw)
		return
	}
	if end {
		rw.Write(q.GetEnd())
		return
	}
	rw.Write(q.GetStart())
}

func answerHandle(rw http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	id := query.Get("id")
	questionnairesLock.RLock()
	q, ok := questionnaires[id]
	questionnairesLock.RUnlock()
	if !ok {
		rw.WriteHeader(http.StatusNotFound)
		translationStruct := translation.GetDefaultTranslation()
		t := errorTemplateStruct{template.HTML(fmt.Sprintf("<h1>%s</h1>", translationStruct.CanNotFindQuestionnaire)), translationStruct, config.ServerPath}
		errorTemplate.Execute(rw, t)
		return
	}
	err := q.SaveData(r)
	if err != nil {
		_, validationError := err.(ErrValidation)
		if validationError {
			log.Printf("server: received bad request (%s)", err.Error())
			rw.WriteHeader(http.StatusBadRequest)
			translationStruct, err := translation.GetTranslation(q.Language)
			if err != nil {
				log.Printf("server: error while getting translation (%s) for questionnaire %s: %s", q.Language, id, err.Error())
				translationStruct = translation.GetDefaultTranslation()
			}
			textTemplate.Execute(rw, textTemplateStruct{template.HTML(translationStruct.ErrorAnswer), translationStruct, config.ServerPath})
			return
		}
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error()))
		return
	}
	http.Redirect(rw, r, fmt.Sprintf("%s/%s?end=1", config.ServerPath, id), http.StatusSeeOther)
}

func resultsHandle(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	translationStruct := translation.GetDefaultTranslation()
	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte(err.Error()))
			return
		}

		key := r.Form.Get("key")
		pw := r.Form.Get("pw")

		questionnairesLock.RLock()
		q, ok := questionnaires[key]
		questionnairesLock.RUnlock()
		if !ok {
			if config.LogFailedLogin {
				log.Printf("Failed login from %s", helper.GetRealIP(r))
			}
			resultsAccessTemplate.Execute(rw, resultsAccessTemplateStruct{translationStruct, config.ServerPath})
			return
		}

		ok, err = registry.ComparePasswords(q.PasswordMethod, pw, q.Password)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte(err.Error()))
			return
		}
		if !ok {
			if config.LogFailedLogin {
				log.Printf("Failed login from %s", helper.GetRealIP(r))
			}
			resultsAccessTemplate.Execute(rw, resultsAccessTemplateStruct{translationStruct, config.ServerPath})
			return
		}

		results, err := q.GetResults()
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte(err.Error()))
			return
		}

		a, err := auth.GetStringsTimed(time.Now(), key)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte(err.Error()))
			return
		}

		translationStruct, err = translation.GetTranslation(q.Language)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte(fmt.Sprintf("can not get translation for language '%s'", q.Language)))
			return
		}

		td := resultsTemplateStruct{
			Results:     results,
			Key:         key,
			Auth:        a,
			Translation: translationStruct,
			ServerPath:  config.ServerPath,
		}

		err = resultsTemplate.ExecuteTemplate(rw, "results.html", td)
		if err != nil {
			fmt.Println(err.Error())
		}

		return
	}
	resultsAccessTemplate.Execute(rw, resultsAccessTemplateStruct{translationStruct, config.ServerPath})
}

func resultDownloadHandle(rw http.ResponseWriter, r *http.Request, filetype string) {
	rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	err := r.ParseForm()
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error()))
		return
	}
	key := r.Form.Get("key")
	a := r.Form.Get("auth")
	pw := r.Form.Get("pw")

	if key == "" {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	if a == "" && pw == "" {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	// We now know either a or pw are not empty
	if a != "" {
		if !auth.VerifyStringsTimed(a, key, time.Now(), 1*time.Hour) {
			rw.WriteHeader(http.StatusUnauthorized)
			rw.Write([]byte("Access key not valid"))
			return
		}
	}

	questionnairesLock.RLock()
	q, ok := questionnaires[key]
	questionnairesLock.RUnlock()

	if !ok {
		rw.WriteHeader(http.StatusUnauthorized)
		if config.LogFailedLogin {
			log.Printf("Failed login from %s", helper.GetRealIP(r))
		}
		return
	}

	if pw != "" {
		ok, err = registry.ComparePasswords(q.PasswordMethod, pw, q.Password)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte(err.Error()))
			return
		}
		if !ok {
			rw.WriteHeader(http.StatusUnauthorized)
			if config.LogFailedLogin {
				log.Printf("Failed login from %s", helper.GetRealIP(r))
			}
			return
		}
	}

	name := strings.ReplaceAll(key, "\"", "_")
	name = strings.ReplaceAll(name, ";", "_")

	rw.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.%s\"", name, filetype))

	switch filetype {
	case "csv":
		err = q.WriteCSV(rw)
	case "zip":
		err = q.WriteZip(rw)
	default:
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(fmt.Sprintf("Unknown filetype %s", filetype)))
		return
	}

	if err != nil {
		log.Printf("error sending %s: %s", filetype, err.Error())
		return
	}
}

func reloadHandle(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	if config.reloadingDisabled {
		rw.WriteHeader(http.StatusNotImplemented)
		tl := translation.GetDefaultTranslation()
		textTemplate.Execute(rw, textTemplateStruct{helper.SanitiseString(fmt.Sprintf("<p>%s</p>", tl.ReloadingDisabled)), tl, config.ServerPath})
		return
	}

	switch r.Method {
	case http.MethodGet:

		reloadTemplate.Execute(rw, resultsAccessTemplateStruct{translation.GetDefaultTranslation(), config.ServerPath})
		return
	case http.MethodPost:

		err := r.ParseForm()
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte(err.Error()))
			return
		}

		isWebsite := r.Form.Get("website") == "true"

		pw := r.Form.Get("pw")
		if pw == "" {
			rw.WriteHeader(http.StatusBadRequest)
			if isWebsite {
				reloadTemplate.Execute(rw, resultsAccessTemplateStruct{translation.GetDefaultTranslation(), config.ServerPath})
				return
			}
			rw.Write([]byte(fmt.Sprintf("no password for reload")))
			return
		}

		validRequest := false
		for i := range config.ReloadPasswords {
			validRequest, err = registry.ComparePasswords(config.ReloadPasswordsMethod, pw, config.ReloadPasswords[i])
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				rw.Write([]byte(err.Error()))
				return
			}
			if validRequest {
				break
			}
		}

		if !validRequest {
			if config.LogFailedLogin {
				log.Printf("Failed login from %s", helper.GetRealIP(r))
			}
			rw.WriteHeader(http.StatusForbidden)
			if isWebsite {
				reloadTemplate.Execute(rw, resultsAccessTemplateStruct{translation.GetDefaultTranslation(), config.ServerPath})
				return
			}
			rw.Write([]byte("403 Forbidden"))
			return
		}

		log.Println("Reloading questionnaires")

		q, err := LoadAllQuestionnaires(config.DataFolder)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			if isWebsite {
				tl := translation.GetDefaultTranslation()
				textTemplate.Execute(rw, textTemplateStruct{helper.SanitiseString(fmt.Sprintf("<p>%s</p>", tl.AnErrorOccured)), tl, config.ServerPath})
				return
			}
			rw.Write([]byte("500 Internal Server Error"))
			log.Println(err)
			return
		}

		questionnairesLock.Lock()
		questionnaires = q
		questionnairesLock.Unlock()

		rw.WriteHeader(http.StatusOK)
		if isWebsite {
			tl := translation.GetDefaultTranslation()
			textTemplate.Execute(rw, textTemplateStruct{helper.SanitiseString(fmt.Sprintf("<p>%s</p>", tl.SurveyReloadSuccessful)), tl, config.ServerPath})
			return
		}

		rw.Write([]byte("200 Ok"))

	default:
		rw.WriteHeader(http.StatusBadRequest)
		rw.Write([]byte(fmt.Sprintf("unknown method %s for reload", r.Method)))
		return
	}
}

// RunServer starts the actual server.
// It does nothing if a server is already started.
// It will return directly after the server is started.
func RunServer() {
	serverMutex.Lock()
	defer serverMutex.Unlock()
	if serverStarted {
		return
	}

	err := initialiseServer()
	if err != nil {
		log.Panicln("server:", err)
	}
	log.Println("server: Server starting at", config.Address)
	serverStarted = true
	go func() {
		err = server.ListenAndServe()
		if err != http.ErrServerClosed {
			log.Println("server:", err)
		}
	}()
}

// StopServer shuts the server down.
// It will do nothing if the server is not started.
// It will return after the shutdown is completed.
func StopServer() {
	serverMutex.Lock()
	defer serverMutex.Unlock()
	if !serverStarted {
		return
	}
	err := server.Shutdown(context.Background())
	if err == nil {
		log.Println("server: stopped")
	} else {
		log.Println("server:", err)
	}
}
