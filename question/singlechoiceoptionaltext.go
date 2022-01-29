// SPDX-License-Identifier: Apache-2.0
// Copyright 2020,2022 Marcus Soll
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
	"strconv"
	"strings"

	"github.com/Top-Ranger/questiongo/helper"
	"github.com/Top-Ranger/questiongo/registry"
)

func init() {
	err := registry.RegisterQuestionType(FactorySingleChoiceOptionalText, "single choice optional text")
	if err != nil {
		panic(err)
	}
}

// FactorySingleChoiceOptionalText is the factory for single choice questions.
func FactorySingleChoiceOptionalText(data []byte, id string, language string) (registry.Question, error) {
	var sc singleChoiceOptionalText
	err := json.Unmarshal(data, &sc)
	if err != nil {
		return nil, err
	}
	sc.id = id

	// Sanity checks
	testID := make(map[string]bool)
	for i := range sc.Answers {
		if len(sc.Answers[i]) != 2 {
			return nil, fmt.Errorf("singlechoiceoptionaltext: Answer %d must have exactly 2 values (id, text) (%s)", i, id)
		}
		if testID[sc.Answers[i][0]] {
			return nil, fmt.Errorf("singlechoiceoptionaltext: ID %s found twice (%s)", sc.Answers[i][0], id)
		}
		if strings.HasPrefix(sc.Answers[i][0], "scot") {
			return nil, fmt.Errorf("singlechoiceoptionaltext: ID %s might not start with 'scot' (%s)", sc.Answers[i][0], id)
		}
		testID[sc.Answers[i][0]] = true
	}

	sc.showTextMap = make(map[string]bool)

	for i := range sc.ShowOptionalText {
		if !testID[sc.ShowOptionalText[i]] {
			return nil, fmt.Errorf("singlechoiceoptionaltext: ID %s for showing text unknown (%s)", sc.ShowOptionalText[i], id)
		}
		sc.showTextMap[sc.ShowOptionalText[i]] = true
	}

	_, ok := registry.GetFormatType(sc.Format)
	if !ok {
		return nil, fmt.Errorf("singlechoiceoptionaltext: Unknown format type %s (%s)", sc.Format, id)
	}

	return &sc, nil
}

var singlechoiceoptionaltextTemplate = template.Must(template.New("singlechoiceoptionaltextTemplate").Parse(`{{.Question}}<br>
{{range $i, $e := .Data }}
<input type="radio" id="{{$e.QID}}_{{$e.AID}}" name="{{$e.QID}}" value="{{$e.AID}}" onchange="if(this.checked){ {{if $e.ShowText}} document.getElementById('{{$e.QID}}_scot_div').removeAttribute('hidden') {{else}} document.getElementById('{{$e.QID}}_scot_div').hidden=true {{end}} }" {{if $.Required}} required {{end}}><label for="{{$e.QID}}_{{$e.AID}}">{{$e.Text}}</label><br>
{{end}}
<div id="{{.QID}}_scot_div" hidden>
<label for="{{.QID}}_scot_text">{{.QuestionOptionalText}}</label><br>
<textarea form="questionnaire" id="{{.QID}}_scot_text" name="{{.QID}}_scot_text" rows="{{.Rows}}"></textarea>
</div>
`))

// TODO
var singlechoiceoptionaltextStatisticsTemplate = template.Must(template.New("singlechoiceoptionaltextStatisticTemplate").Parse(`{{.Question}}<br>
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
<br>
{{.QuestionOptionalText}}
<details>
<summary>show results ({{len .TextData}}, {{.PercentShown}}%)</summary>
<ol>
{{range $i, $e := .TextData }}
<li>{{$e}}</li>
{{end}}
</ol>
</details>
`))

type singlechoiceoptionaltextTemplateStructInner struct {
	QID      string
	AID      string
	Text     template.HTML
	ShowText bool
}

type singlechoiceoptionaltextStatisticTemplateStruct struct {
	Question             template.HTML
	Data                 []singlechoiceoptionaltextStatisticsTemplateStructInner
	Image                template.HTML
	QuestionOptionalText template.HTML
	TextData             []string
	PercentShown         int
}

type singlechoiceoptionaltextStatisticsTemplateStructInner struct {
	Question template.HTML
	Result   float64
}

type singlechoiceoptionaltextTemplateStruct struct {
	QID                  string
	Question             template.HTML
	Required             bool
	Data                 []singlechoiceoptionaltextTemplateStructInner
	QuestionOptionalText template.HTML
	Rows                 int
}

type singleChoiceOptionalTextResult struct {
	Answer    string
	TextShown bool
	Text      string `json:",omitempty"`
}

type singleChoiceOptionalText struct {
	Random               bool
	Required             bool
	Format               string
	Question             string
	Answers              [][]string
	QuestionOptionalText string
	RowsOptionalText     int
	ShowOptionalText     []string

	id          string
	showTextMap map[string]bool
}

func (sc singleChoiceOptionalText) GetID() string {
	return sc.id
}

func (sc singleChoiceOptionalText) GetHTML() template.HTML {
	f, _ := registry.GetFormatType(sc.Format)
	td := singlechoiceoptionaltextTemplateStruct{
		QID:                  sc.id,
		Question:             f.Format([]byte(sc.Question)),
		Required:             sc.Required,
		Data:                 make([]singlechoiceoptionaltextTemplateStructInner, 0, len(sc.Answers)),
		QuestionOptionalText: f.Format([]byte(sc.QuestionOptionalText)),
		Rows:                 sc.RowsOptionalText,
	}
	for i := range sc.Answers {
		scts := singlechoiceoptionaltextTemplateStructInner{
			QID:      sc.id,
			AID:      sc.Answers[i][0],
			Text:     f.FormatClean([]byte(sc.Answers[i][1])),
			ShowText: sc.showTextMap[sc.Answers[i][0]],
		}
		td.Data = append(td.Data, scts)
	}

	if sc.Random {
		rand.Shuffle(len(td.Data), func(i, j int) {
			td.Data[i], td.Data[j] = td.Data[j], td.Data[i]
		})
	}

	output := bytes.NewBuffer(make([]byte, 0))
	err := singlechoiceoptionaltextTemplate.Execute(output, td)
	if err != nil {
		log.Printf("singlechoiceoptionaltext: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (sc singleChoiceOptionalText) GetStatisticsHeader() []string {
	return []string{sc.id, fmt.Sprintf("%s_textShown", sc.id), fmt.Sprintf("%s_text", sc.id)}
}

func (sc singleChoiceOptionalText) GetStatistics(data []string) [][]string {
	result := make([][]string, len(data))
	for d := range data {
		var r singleChoiceOptionalTextResult
		result[d] = make([]string, 3)
		err := json.Unmarshal([]byte(data[d]), &r)
		if err != nil {
			result[d][0] = "[ERROR]"
			result[d][1] = "[ERROR]"
			result[d][2] = "[ERROR]"
			return result
		}
		result[d][0] = r.Answer
		result[d][1] = strconv.FormatBool(r.TextShown)
		result[d][2] = r.Text
	}
	return result
}

func (sc singleChoiceOptionalText) GetStatisticsDisplay(data []string) template.HTML {
	count := 0
	countAnswer := make([]int, len(sc.Answers)+1)

	f, _ := registry.GetFormatType(sc.Format)
	td := singlechoiceoptionaltextStatisticTemplateStruct{
		Question:             f.Format([]byte(sc.Question)),
		Data:                 make([]singlechoiceoptionaltextStatisticsTemplateStructInner, 0, len(sc.Answers)+1),
		QuestionOptionalText: f.Format([]byte(sc.QuestionOptionalText)),
		TextData:             make([]string, 0, len(sc.Answers)),
		PercentShown:         0,
	}

	for d := range data {
		count++
		found := false
		var r singleChoiceOptionalTextResult
		err := json.Unmarshal([]byte(data[d]), &r)
		if err != nil {
			log.Printf("singlechoiceoptionaltext: Can not parse '%s':  %s (%s)", data[d], err.Error(), sc.id)
			continue
		}
		if r.TextShown {
			td.TextData = append(td.TextData, r.Text)
		}
		for i := range sc.Answers {
			if r.Answer == sc.Answers[i][0] {
				found = true
				countAnswer[i]++
				break
			}
		}
		if !found {
			countAnswer[len(sc.Answers)]++
		}
	}

	td.PercentShown = 100 * len(td.TextData) / len(data)

	v := make([]helper.ChartValue, len(sc.Answers)+1)
	for i := range sc.Answers {
		question := f.FormatClean([]byte(sc.Answers[i][1]))
		v[i].Label = string(helper.SanitiseStringClean(string(question)))
		v[i].Value = float64(countAnswer[i])
		inner := singlechoiceoptionaltextStatisticsTemplateStructInner{
			Question: question,
			Result:   float64(countAnswer[i]) / float64(count),
		}
		td.Data = append(td.Data, inner)
	}
	{
		v[len(sc.Answers)].Label = "[no answer]"
		v[len(sc.Answers)].Value = float64(countAnswer[len(sc.Answers)])
		inner := singlechoiceoptionaltextStatisticsTemplateStructInner{
			Question: "[no answer]",
			Result:   float64(countAnswer[len(sc.Answers)]) / float64(count),
		}
		td.Data = append(td.Data, inner)
	}

	td.Image = helper.PieChart(v, sc.id, string(f.FormatClean([]byte(sc.Question))))

	output := bytes.NewBuffer(make([]byte, 0))
	err := singlechoiceoptionaltextStatisticsTemplate.Execute(output, td)
	if err != nil {
		log.Printf("singlechoiceoptionaltext: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (sc singleChoiceOptionalText) ValidateInput(data map[string][]string) error {
	r, ok := data[sc.id]
	if !ok {
		if sc.Required {
			return fmt.Errorf("singlechoiceoptionaltext: Required, but no input found")
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
	return fmt.Errorf("singlechoiceoptionaltext: Unknown id '%s'", r[0])
}

func (sc singleChoiceOptionalText) IgnoreRecord(data map[string][]string) bool {
	return false
}

func (sc singleChoiceOptionalText) GetDatabaseEntry(data map[string][]string) string {
	result := singleChoiceOptionalTextResult{}
	r, ok := data[sc.id]
	if ok && len(r) == 1 {
		result.Answer = r[0]
	}

	result.TextShown = sc.showTextMap[result.Answer]
	if result.TextShown {
		r, ok = data[fmt.Sprintf("%s_scot_text", sc.id)]
		if ok && len(r) == 1 {
			result.Text = r[0]
		}
	}

	b, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf("ERROR: %s", err.Error())
	}
	return string(b)
}
