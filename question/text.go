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

package question

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"

	"github.com/Top-Ranger/questiongo/registry"
)

func init() {
	err := registry.RegisterQuestionType(FactoryText, "text")
	if err != nil {
		panic(err)
	}
}

// FactoryText is the factory for text questions.
func FactoryText(data []byte, id string) (registry.Question, error) {
	var t text
	err := json.Unmarshal(data, &t)
	if err != nil {
		return nil, err
	}
	t.id = id

	_, ok := registry.GetFormatType(t.Format)
	if !ok {
		return nil, fmt.Errorf("text: Unknown format type %s (%s)", t.Format, id)
	}

	return &t, nil
}

var textTemplate = template.Must(template.New("textTemplate").Parse(`<label for="{{.QID}}">{{.Question}}</label><br>
<textarea form="questionnaire" id="{{.QID}}" name="{{.QID}}" rows="{{.Rows}}" {{if .Required}} required {{end}}></textarea>
`))

var textStatisticsTemplate = template.Must(template.New("textStatisticTemplate").Parse(`{{.Question}}
<ol>
{{range $i, $e := .Data }}
<li>{{$e}}</li>
{{end}}
</ol>
`))

type textTemplateStruct struct {
	Question template.HTML
	QID      string
	Rows     int
	Required bool
}

type textStatisticTemplateStruct struct {
	Question template.HTML
	Data     []string
}

type text struct {
	Format   string
	Question string
	Lines    int
	Required bool

	id string
}

func (t text) GetID() string {
	return t.id
}

func (t text) GetHTML() template.HTML {
	f, _ := registry.GetFormatType(t.Format)

	td := textTemplateStruct{
		Question: f.Format([]byte(t.Question)),
		QID:      t.id,
		Rows:     t.Lines,
		Required: t.Required,
	}

	output := bytes.NewBuffer(make([]byte, 0))
	err := textTemplate.Execute(output, td)
	if err != nil {
		log.Printf("text: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (t text) GetStatisticsHeader() []string {
	return []string{t.id}
}

func (t text) GetStatistics(data []string) [][]string {
	result := make([][]string, len(data))
	for i := range data {
		result[i] = []string{data[i]}
	}
	return result
}

func (t text) GetStatisticsDisplay(data []string) template.HTML {
	f, _ := registry.GetFormatType(t.Format)
	answer := make([]string, 0, len(data))

	for i := range data {
		if data[i] != "" {
			answer = append(answer, data[i])
		}
	}

	td := textStatisticTemplateStruct{
		Question: f.Format([]byte(t.Question)),
		Data:     answer,
	}

	output := bytes.NewBuffer(make([]byte, 0))
	err := textStatisticsTemplate.Execute(output, td)
	if err != nil {
		log.Printf("text: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (t text) ValidateInput(data map[string][]string) error {
	if t.Required && len(data[t.id]) == 0 {
		return fmt.Errorf("text: Required, but no input found")
	}
	if len(data[t.id][0]) == 0 {
		return fmt.Errorf("text: Required, but no input found")
	}
	return nil
}

func (t text) GetDatabaseEntry(data map[string][]string) string {
	if len(data[t.id]) >= 1 {
		return data[t.id][0]
	}
	return ""
}
