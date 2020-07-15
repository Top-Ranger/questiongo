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
	"math/rand"

	"github.com/Top-Ranger/questiongo/helper"
	"github.com/Top-Ranger/questiongo/registry"
)

func init() {
	err := registry.RegisterQuestionType(FactorySingleChoice, "single choice")
	if err != nil {
		panic(err)
	}
}

// FactorySingleChoice is the factory for single choice questions.
func FactorySingleChoice(data []byte, id string, language string) (registry.Question, error) {
	var sc singleChoice
	err := json.Unmarshal(data, &sc)
	if err != nil {
		return nil, err
	}
	sc.id = id

	// Sanity checks
	testID := make(map[string]bool)
	for i := range sc.Answers {
		if len(sc.Answers[i]) != 2 {
			return nil, fmt.Errorf("singlechoice: Answer %d must have exactly 2 values (id, text) (%s)", i, id)
		}
		if testID[sc.Answers[i][0]] {
			return nil, fmt.Errorf("singlechoice: ID %s found twice (%s)", sc.Answers[i][0], id)
		}
		testID[sc.Answers[i][0]] = true
	}

	_, ok := registry.GetFormatType(sc.Format)
	if !ok {
		return nil, fmt.Errorf("singlechoice: Unknown format type %s (%s)", sc.Format, id)
	}

	return &sc, nil
}

var singlechoiceTemplate = template.Must(template.New("singlechoiceTemplate").Parse(`{{.Question}}<br>
{{range $i, $e := .Data }}
<input type="radio" id="{{$e.QID}}_{{$e.AID}}" name="{{$e.QID}}" value="{{$e.AID}}" {{if $.Required}} required {{end}}><label for="{{$e.QID}}_{{$e.AID}}">{{$e.Text}}</label><br>
{{end}}`))

var singlechoiceStatisticsTemplate = template.Must(template.New("singlechoiceStatisticTemplate").Parse(`{{.Question}}<br>
<table>
<thead>
<tr>
<th>Question</th>
<th>Answer (percentage)</th>
</tr>
</thead>
{{range $i, $e := .Data }}
<tr>
<td>{{$e.Question}}</td>
<td>{{printf "%.2f" $e.Result}}</td>
</tr>
{{end}}
</tbody>
</table>
<br>
{{.Image}}
`))

type singlechoiceTemplateStructInner struct {
	QID  string
	AID  string
	Text template.HTML
}

type singlechoiceStatisticTemplateStruct struct {
	Question template.HTML
	Data     []singlechoiceStatisticsTemplateStructInner
	Image    template.HTML
}

type singlechoiceStatisticsTemplateStructInner struct {
	Question template.HTML
	Result   float64
}

type singlechoiceTemplateStruct struct {
	Question template.HTML
	Required bool
	Data     []singlechoiceTemplateStructInner
}

type singleChoice struct {
	Random   bool
	Required bool
	Format   string
	Question string
	Answers  [][]string

	id string
}

func (sc singleChoice) GetID() string {
	return sc.id
}

func (sc singleChoice) GetHTML() template.HTML {
	f, _ := registry.GetFormatType(sc.Format)
	td := singlechoiceTemplateStruct{
		Question: f.Format([]byte(sc.Question)),
		Required: sc.Required,
		Data:     make([]singlechoiceTemplateStructInner, 0, len(sc.Answers)),
	}
	for i := range sc.Answers {
		scts := singlechoiceTemplateStructInner{
			QID:  sc.id,
			AID:  sc.Answers[i][0],
			Text: f.FormatClean([]byte(sc.Answers[i][1])),
		}
		td.Data = append(td.Data, scts)
	}

	if sc.Random {
		rand.Shuffle(len(td.Data), func(i, j int) {
			td.Data[i], td.Data[j] = td.Data[j], td.Data[i]
		})
	}

	output := bytes.NewBuffer(make([]byte, 0))
	err := singlechoiceTemplate.Execute(output, td)
	if err != nil {
		log.Printf("singlechoice: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (sc singleChoice) GetStatisticsHeader() []string {
	return []string{sc.id}
}

func (sc singleChoice) GetStatistics(data []string) [][]string {
	result := make([][]string, len(data))
	for d := range data {
		result[d] = make([]string, 1)
		result[d][0] = data[d]
	}
	return result
}

func (sc singleChoice) GetStatisticsDisplay(data []string) template.HTML {
	count := 0
	countAnswer := make([]int, len(sc.Answers)+1)

	for d := range data {
		count++
		found := false
		for i := range sc.Answers {
			if data[d] == sc.Answers[i][0] {
				found = true
				countAnswer[i]++
				break
			}
		}
		if !found {
			countAnswer[len(sc.Answers)]++
		}
	}

	f, _ := registry.GetFormatType(sc.Format)
	td := singlechoiceStatisticTemplateStruct{
		Question: f.Format([]byte(sc.Question)),
		Data:     make([]singlechoiceStatisticsTemplateStructInner, 0, len(sc.Answers)+1),
	}
	v := make([]helper.ChartValue, len(sc.Answers)+1)
	for i := range sc.Answers {
		question := f.FormatClean([]byte(sc.Answers[i][1]))
		v[i].Label = string(helper.SanitiseStringClean(string(question)))
		v[i].Value = float64(countAnswer[i])
		inner := singlechoiceStatisticsTemplateStructInner{
			Question: question,
			Result:   float64(countAnswer[i]) / float64(count),
		}
		td.Data = append(td.Data, inner)
	}
	{
		v[len(sc.Answers)].Label = "[no answer]"
		v[len(sc.Answers)].Value = float64(countAnswer[len(sc.Answers)])
		inner := singlechoiceStatisticsTemplateStructInner{
			Question: "[no answer]",
			Result:   float64(countAnswer[len(sc.Answers)]) / float64(count),
		}
		td.Data = append(td.Data, inner)
	}

	td.Image = helper.PieChart(v, sc.id, string(f.FormatClean([]byte(sc.Question))))

	output := bytes.NewBuffer(make([]byte, 0))
	err := singlechoiceStatisticsTemplate.Execute(output, td)
	if err != nil {
		log.Printf("singlechoice: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (sc singleChoice) ValidateInput(data map[string][]string) error {
	r, ok := data[sc.id]
	if !ok {
		if sc.Required {
			return fmt.Errorf("singlechoice: Required, but no input found")
		}
		return nil
	}

	if len(r) != 1 {
		return fmt.Errorf("sindlechoice: Malformed input")
	}
	for i := range sc.Answers {
		if r[0] == sc.Answers[i][0] {
			return nil
		}
	}
	return fmt.Errorf("singlechoice: Unknown id '%s'", r[0])
}

func (sc singleChoice) IgnoreRecord(data map[string][]string) bool {
	return false
}

func (sc singleChoice) GetDatabaseEntry(data map[string][]string) string {
	result := ""
	r, ok := data[sc.id]
	if ok && len(r) == 1 {
		result = r[0]
	}
	return result
}
