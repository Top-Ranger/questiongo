// SPDX-License-Identifier: Apache-2.0
// Copyright 2020 Marcus Soll
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
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
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

var textTemplate *template.Template
var errorTemplate *template.Template
var resultsTemplate *template.Template
var resultsAccessTemplate *template.Template

var dsgvo []byte
var impressum []byte

var questionnaires map[string]Questionnaire

var cachedFiles = make(map[string][]byte, 50)
var etagCompare string

var robottxt = []byte(`User-agent: *
Disallow: /`)

func init() {
	b, err := ioutil.ReadFile("template/error.html")
	if err != nil {
		panic(err)
	}

	errorTemplate, err = template.New("error").Parse(string(b))
	if err != nil {
		panic(err)
	}

	b, err = ioutil.ReadFile("template/text.html")
	if err != nil {
		panic(err)
	}

	textTemplate, err = template.New("text").Parse(string(b))
	if err != nil {
		panic(err)
	}

	b, err = ioutil.ReadFile("template/resultsAccess.html")
	if err != nil {
		panic(err)
	}
	resultsAccessTemplate, err = template.New("text").Parse(string(b))
	if err != nil {
		panic(err)
	}

	funcMap := template.FuncMap{
		"even": func(i int) bool {
			return i%2 == 0
		},
	}

	b, err = ioutil.ReadFile("template/results.html")
	if err != nil {
		panic(err)
	}

	resultsTemplate, err = template.New("results").Funcs(funcMap).Parse(string(b))
	if err != nil {
		panic(err)
	}
}

type errorTemplateStruct struct {
	Error       template.HTML
	Translation translation.Translation
}

type textTemplateStruct struct {
	Text        template.HTML
	Translation translation.Translation
}

type resultsTemplateStruct struct {
	Results     []template.HTML
	Key         string
	Auth        string
	Translation translation.Translation
}

type resultsAccessTemplateStruct struct {
	Translation translation.Translation
}

func initialiseServer() error {
	if serverStarted {
		return nil
	}
	server = http.Server{Addr: config.Address}

	// Do setup
	// DSGVO
	b, err := ioutil.ReadFile(config.PathDSGVO)
	if err != nil {
		return err
	}
	f, ok := registry.GetFormatType(config.FormatDSGVO)
	if !ok {
		return fmt.Errorf("Unknown format type %s (DSGVO)", config.FormatDSGVO)
	}
	text := textTemplateStruct{f.Format(b), translation.GetDefaultTranslation()}
	output := bytes.NewBuffer(make([]byte, 0, len(text.Text)*2))
	textTemplate.Execute(output, text)
	dsgvo = output.Bytes()

	http.HandleFunc("/dsgvo.html", func(rw http.ResponseWriter, r *http.Request) {
		rw.Write(dsgvo)
	})

	// Impressum
	b, err = ioutil.ReadFile(config.PathImpressum)
	if err != nil {
		return err
	}
	f, ok = registry.GetFormatType(config.FormatImpressum)
	if !ok {
		return fmt.Errorf("Unknown format type %s (impressum)", config.FormatImpressum)
	}
	text = textTemplateStruct{f.Format(b), translation.GetDefaultTranslation()}
	text.Text = template.HTML(strings.Join([]string{string(text.Text), "<p><img style=\"max-width: 500px\" src=\"/static/Logo.svg\" alt=\"Logo\"></p>"}, ""))
	output = bytes.NewBuffer(make([]byte, 0, len(text.Text)*2))
	textTemplate.Execute(output, text)
	impressum = output.Bytes()
	http.HandleFunc("/impressum.html", func(rw http.ResponseWriter, r *http.Request) {
		rw.Write(impressum)
	})

	// static files
	for _, d := range []string{"css/", "static/", "font/", "js/"} {
		filepath.Walk(d, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				log.Panicln("server: Error wile caching files:", err)
			}

			if info.Mode().IsRegular() {
				log.Println("static file handler: Caching file", path)

				b, err := ioutil.ReadFile(path)
				if err != nil {
					log.Println("static file handler: Error reading file:", err)
					return err
				}
				cachedFiles[path] = b
				return nil
			}
			return nil
		})
	}

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
		path = strings.TrimPrefix(path, "/")
		data, ok := cachedFiles[path]
		if !ok {
			rw.WriteHeader(http.StatusNotFound)
		} else {
			rw.Header().Set("ETag", etag)
			rw.Header().Set("Cache-Control", "public, max-age=43200")
			switch {
			case strings.HasSuffix(path, ".svg"):
				rw.Header().Set("Content-Type", "image/svg+xml")
			case strings.HasSuffix(path, ".css"):
				rw.Header().Set("Content-Type", "text/css")
			case strings.HasSuffix(path, ".ttf"):
				rw.Header().Set("Content-Type", "application/x-font-truetype")
			case strings.HasSuffix(path, ".js"):
				rw.Header().Set("Content-Type", "application/javascript")
			default:
				rw.Header().Set("Content-Type", "text/plain")
			}
			rw.Write(data)
		}
	}

	http.HandleFunc("/css/", staticHandle)
	http.HandleFunc("/static/", staticHandle)
	http.HandleFunc("/font/", staticHandle)
	http.HandleFunc("/js/", staticHandle)

	// robots.txt
	http.HandleFunc("/robots.txt", func(rw http.ResponseWriter, r *http.Request) {
		rw.Write(robottxt)
	})

	// Questionnaires
	questionnaires, err = LoadAllQuestionnaires(config.DataFolder)
	if err != nil {
		return err
	}
	http.HandleFunc("/answer.html", answerHandle)
	http.HandleFunc("/results.html", resultsHandle)
	http.HandleFunc("/results.zip", zipHandle)
	http.HandleFunc("/results.csv", csvHandle)
	http.HandleFunc("/", questionnaireHandle)

	return nil
}

func questionnaireHandle(rw http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		t := errorTemplateStruct{"<h1>QuestionGo!</h1>", translation.GetDefaultTranslation()}
		errorTemplate.Execute(rw, t)
		return
	}

	key := r.URL.Path
	key = strings.TrimLeft(key, "/")
	q, ok := questionnaires[key]
	if !ok {
		rw.WriteHeader(http.StatusNotFound)
		translationStruct := translation.GetDefaultTranslation()
		t := errorTemplateStruct{template.HTML(fmt.Sprintf("<h1>%s</h1>", translationStruct.CanNotFindQuestionnaire)), translationStruct}
		errorTemplate.Execute(rw, t)
		return
	}

	if !q.Open {
		translationStruct, err := translation.GetTranslation(q.Language)
		if err != nil {
			log.Printf("server: error while getting translation (%s) for questionnaire %s: %s", q.Language, key, err.Error())
			translationStruct = translation.GetDefaultTranslation()
		}

		t := errorTemplateStruct{helper.SanitiseString(fmt.Sprintf(translationStruct.QuestionnaireClosed, q.Contact)), translationStruct}
		errorTemplate.Execute(rw, t)
		return
	}
	query := r.URL.Query()
	_, main := query["main"]
	_, end := query["end"]

	if main {
		rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		q.WriteQuestions(rw)
		return
	}
	if end {
		rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		rw.Write(q.GetEnd())
		return
	}
	rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	rw.Write(q.GetStart())
	return
}

func answerHandle(rw http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	id := query.Get("id")
	q, ok := questionnaires[id]
	if !ok {
		rw.WriteHeader(http.StatusNotFound)
		translationStruct := translation.GetDefaultTranslation()
		t := errorTemplateStruct{template.HTML(fmt.Sprintf("<h1>%s</h1>", translationStruct.CanNotFindQuestionnaire)), translationStruct}
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
			textTemplate.Execute(rw, textTemplateStruct{template.HTML(translationStruct.ErrorAnswer), translationStruct})
			return
		}
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error()))
		return
	}
	http.Redirect(rw, r, fmt.Sprintf("%s?end=1", id), http.StatusSeeOther)
}

func resultsHandle(rw http.ResponseWriter, r *http.Request) {
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

		q, ok := questionnaires[key]
		if !ok {
			rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			resultsAccessTemplate.Execute(rw, resultsAccessTemplateStruct{translationStruct})
			return
		}

		if !q.VerifyPassword(pw) {
			rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			resultsAccessTemplate.Execute(rw, resultsAccessTemplateStruct{translationStruct})
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

		td := resultsTemplateStruct{
			Results:     results,
			Key:         key,
			Auth:        a,
			Translation: translationStruct,
		}

		rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		resultsTemplate.Execute(rw, td)

		return
	}
	rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	resultsAccessTemplate.Execute(rw, resultsAccessTemplateStruct{translationStruct})
}

func zipHandle(rw http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error()))
		return
	}
	key := r.Form.Get("key")
	a := r.Form.Get("auth")

	if !auth.VerifyStringsTimed(a, key, time.Now(), 1*time.Hour) {
		rw.WriteHeader(http.StatusUnauthorized)
		rw.Write([]byte("Access key not valid"))
		return
	}

	q, ok := questionnaires[key]
	if !ok {
		rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		resultsAccessTemplate.Execute(rw, resultsAccessTemplateStruct{translation.GetDefaultTranslation()})
		return
	}

	rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	err = q.WriteZip(rw)
	if err != nil {
		log.Printf("error sending zip: %s", err.Error())
		return
	}
}

func csvHandle(rw http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error()))
		return
	}
	key := r.Form.Get("key")
	a := r.Form.Get("auth")

	if !auth.VerifyStringsTimed(a, key, time.Now(), 1*time.Hour) {
		rw.WriteHeader(http.StatusUnauthorized)
		rw.Write([]byte("Access key not valid"))
		return
	}

	q, ok := questionnaires[key]
	if !ok {
		rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		resultsAccessTemplate.Execute(rw, resultsAccessTemplateStruct{translation.GetDefaultTranslation()})
		return
	}

	rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	err = q.WriteCSV(rw)
	if err != nil {
		log.Printf("error sending zip: %s", err.Error())
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
