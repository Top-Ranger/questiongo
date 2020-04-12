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
	"strings"

	"github.com/Top-Ranger/questiongo/helper"
	"github.com/Top-Ranger/questiongo/registry"
)

func init() {
	err := registry.RegisterQuestionType(FactoryMultipleChoice, "multiple choice")
	if err != nil {
		panic(err)
	}
}

// FactoryMultipleChoice is the factory for multiple choice questions.
func FactoryMultipleChoice(data []byte, id string) (registry.Question, error) {
	var mc multipleChoice
	err := json.Unmarshal(data, &mc)
	if err != nil {
		return nil, err
	}
	mc.id = id

	// Sanity checks
	testID := make(map[string]bool)
	for i := range mc.Answers {
		if len(mc.Answers[i]) != 2 {
			return nil, fmt.Errorf("multiplechoice: Answer %d must have exactly 2 values (id, text) (%s)", i, id)
		}
		if testID[mc.Answers[i][0]] {
			return nil, fmt.Errorf("multiplechoice: ID %s found twice (%s)", mc.Answers[i][0], id)
		}
		testID[mc.Answers[i][0]] = true
	}

	_, ok := registry.GetFormatType(mc.Format)
	if !ok {
		return nil, fmt.Errorf("multiplechoice: Unknown format type %s (%s)", mc.Format, id)
	}

	return &mc, nil
}

var multiplechoiceTemplate = template.Must(template.New("multiplechoiceTemplate").Parse(`{{.Question}}<br>
{{range $i, $e := .Data }}
<input type="checkbox" id="{{$e.QID}}_{{$e.AID}}" name="{{$e.QID}}_{{$e.AID}}"><label for="{{$e.QID}}_{{$e.AID}}">{{$e.Text}}</label><br>
{{end}}`))

var multiplechoiceStatisticsTemplate = template.Must(template.New("multiplechoiceStatisticTemplate").Parse(`{{.Question}}<br>
<table>
<thead>
<tr>
<th>Question</th>
<th>Answer (percentage)</th>
</tr>
</thead>
<tbody>
{{range $i, $e := .Data }}
<tr>
<td>{{$e.Question}}</td>
<td>{{$e.Result}}</td>
</tr>
{{end}}
<tr>
<td class="th-cell">[number answers]</td>
<td>{{.Sum}}</td>
</tr>
</tbody>
</table>
<br>
{{.Image}}
`))

type multiplechoiceTemplateStructInner struct {
	QID  string
	AID  string
	Text template.HTML
}

type multiplechoiceStatisticTemplateStruct struct {
	Question template.HTML
	Sum      int
	Data     []multiplechoiceStatisticsTemplateStructInner
	Image    template.HTML
}

type multiplechoiceStatisticsTemplateStructInner struct {
	Question template.HTML
	Result   int
}

type multiplechoiceTemplateStruct struct {
	Question template.HTML
	Data     []multiplechoiceTemplateStructInner
}

type multipleChoice struct {
	Random   bool
	Format   string
	Question string
	Answers  [][]string

	id string
}

func (mc multipleChoice) GetID() string {
	return mc.id
}

func (mc multipleChoice) GetHTML() template.HTML {
	f, _ := registry.GetFormatType(mc.Format)
	td := multiplechoiceTemplateStruct{
		Question: f.Format([]byte(mc.Question)),
		Data:     make([]multiplechoiceTemplateStructInner, 0, len(mc.Answers)),
	}
	for i := range mc.Answers {
		mcts := multiplechoiceTemplateStructInner{
			QID:  mc.id,
			AID:  mc.Answers[i][0],
			Text: f.FormatClean([]byte(mc.Answers[i][1])),
		}
		td.Data = append(td.Data, mcts)
	}

	if mc.Random {
		rand.Shuffle(len(td.Data), func(i, j int) {
			td.Data[i], td.Data[j] = td.Data[j], td.Data[i]
		})
	}

	output := bytes.NewBuffer(make([]byte, 0))
	err := multiplechoiceTemplate.Execute(output, td)
	if err != nil {
		log.Printf("multiplechoice: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (mc multipleChoice) GetStatisticsHeader() []string {
	header := make([]string, len(mc.Answers))
	for i := range mc.Answers {
		header[i] = fmt.Sprintf("%s_%s", mc.id, mc.Answers[i][0])
	}
	return header
}

func (mc multipleChoice) GetStatistics(data []string) [][]string {
	result := make([][]string, len(data))
	for d := range data {
		r := make([]string, len(mc.Answers))
		errorState := true
		if !strings.HasPrefix(data[d], "ERROR") {
			boolarray := make([]bool, len(mc.Answers))
			err := json.Unmarshal([]byte(data[d]), &boolarray)
			if err == nil && len(boolarray) == len(r) {
				errorState = false
				for i := range boolarray {
					if boolarray[i] {
						r[i] = "true"
					} else {
						r[i] = "false"
					}
				}
			}
		}

		if errorState {
			for i := range r {
				r[i] = "error"
			}
		}
		result[d] = r
	}
	return result
}

func (mc multipleChoice) GetStatisticsDisplay(data []string) template.HTML {
	count := 0
	countAnswer := make([]int, len(mc.Answers))

	for d := range data {
		boolarray := make([]bool, len(mc.Answers))
		err := json.Unmarshal([]byte(data[d]), &boolarray)
		if err != nil {
			continue
		}
		if len(boolarray) != len(mc.Answers) {
			continue
		}
		count++
		for i := range mc.Answers {
			if boolarray[i] {
				countAnswer[i]++
			}
		}
	}

	f, _ := registry.GetFormatType(mc.Format)
	td := multiplechoiceStatisticTemplateStruct{
		Question: f.Format([]byte(mc.Question)),
		Sum:      count,
		Data:     make([]multiplechoiceStatisticsTemplateStructInner, 0, len(mc.Answers)),
	}
	v := make([]helper.ChartValue, len(mc.Answers))
	for i := range mc.Answers {
		question := f.FormatClean([]byte(mc.Answers[i][1]))
		v[i].Label = string(question)
		v[i].Value = float64(countAnswer[i])
		inner := multiplechoiceStatisticsTemplateStructInner{
			Question: question,
			Result:   countAnswer[i],
		}
		td.Data = append(td.Data, inner)
	}

	td.Image = helper.BarChart(v, mc.id, string(f.FormatClean([]byte(mc.Question))))

	output := bytes.NewBuffer(make([]byte, 0))
	err := multiplechoiceStatisticsTemplate.Execute(output, td)
	if err != nil {
		log.Printf("multiplechoice: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (mc multipleChoice) ValidateInput(data map[string][]string) error {
	return nil
}

func (mc multipleChoice) GetDatabaseEntry(data map[string][]string) string {
	result := make([]bool, len(mc.Answers))
	for i := range mc.Answers {
		_, ok := data[fmt.Sprintf("%s_%s", mc.id, mc.Answers[i][0])]
		if ok {
			result[i] = true
		} else {
			result[i] = false
		}
	}
	b, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf("ERROR: %s", err.Error())
	}
	return string(b)
}
