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
	"sort"
	"strings"
	"time"

	"github.com/Top-Ranger/questiongo/helper"
	"github.com/Top-Ranger/questiongo/registry"
)

func init() {
	err := registry.RegisterQuestionType(FactoryTime, "time")
	if err != nil {
		panic(err)
	}
}

// FactoryTime is the factory for time questions.
func FactoryTime(data []byte, id string) (registry.Question, error) {
	var t timeQuestion
	err := json.Unmarshal(data, &t)
	if err != nil {
		return nil, err
	}
	t.id = id

	_, ok := registry.GetFormatType(t.Format)
	if !ok {
		return nil, fmt.Errorf("time: Unknown format type %s (%s)", t.Format, id)
	}

	return &t, nil
}

var timeTemplate = template.Must(template.New("timeTemplate").Parse(`<label for="{{.QID}}">{{.Question}}</label><br>
<input type="time" id="{{.QID}}" name="{{.QID}}" placeholder="yyyy-mm-dd" pattern="^\d{4}-\d{2}-\d{2}$" {{if .Required}} required {{end}}>
`))

var timeStatisticsTemplate = template.Must(template.New("timeStatisticTemplate").Parse(`{{.Question}}<br>
<table>
<thead>
<tr>
<th>Time</th>
<th>Number</th>
</tr>
</thead>
{{range $i, $e := .Data }}
<tr>
<td {{if $e.Special}}class="th-cell"{{end}}>{{$e.Time}}</td>
<td>{{$e.Number}}</td>
</tr>
{{end}}
</tbody>
</table>
<br>
{{.Image}}
`))

type timeTemplateStruct struct {
	Question template.HTML
	QID      string
	Required bool
}

type timeStatisticTemplateStructInner struct {
	Time    string
	Number  int
	Special bool
}

type timeStatisticTemplateStruct struct {
	Question template.HTML
	Data     []timeStatisticTemplateStructInner
	Image    template.HTML
}
type timeStatisticTemplateStructInnerSort []timeStatisticTemplateStructInner

func (d timeStatisticTemplateStructInnerSort) Len() int {
	return len(d)
}

func (d timeStatisticTemplateStructInnerSort) Less(i, j int) bool {
	return d[i].Time < d[j].Time
}

func (d timeStatisticTemplateStructInnerSort) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

type timeQuestion struct {
	Format   string
	Question string
	Required bool

	id string
}

func (t timeQuestion) GetID() string {
	return t.id
}

func (t timeQuestion) GetHTML() template.HTML {
	f, _ := registry.GetFormatType(t.Format)

	td := timeTemplateStruct{
		Question: f.Format([]byte(t.Question)),
		QID:      t.id,
		Required: t.Required,
	}

	output := bytes.NewBuffer(make([]byte, 0))
	err := timeTemplate.Execute(output, td)
	if err != nil {
		log.Printf("time: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (t timeQuestion) GetStatisticsHeader() []string {
	return []string{t.id}
}

func (t timeQuestion) GetStatistics(data []string) [][]string {
	result := make([][]string, len(data))
	for i := range data {
		result[i] = []string{data[i]}
	}
	return result
}

func (t timeQuestion) GetStatisticsDisplay(data []string) template.HTML {
	f, _ := registry.GetFormatType(t.Format)
	answer := make(map[string]int)

	for i := range data {
		if data[i] == "" {
			answer["[no answer]"]++
		} else if strings.HasPrefix(data[i], "[invalid input]") {
			answer["[invalid input]"]++
		} else {
			answer[data[i]]++
		}
	}

	td := timeStatisticTemplateStruct{
		Question: f.Format([]byte(t.Question)),
		Data:     make([]timeStatisticTemplateStructInner, 0, len(answer)),
	}

	for k := range answer {
		td.Data = append(td.Data, timeStatisticTemplateStructInner{Time: k, Number: answer[k], Special: strings.HasPrefix(string(k), "[")})
	}

	sort.Sort(timeStatisticTemplateStructInnerSort(td.Data))

	v := make([]helper.ChartValue, len(td.Data))
	for i := range td.Data {
		v[i].Label = td.Data[i].Time
		v[i].Value = float64(td.Data[i].Number)
	}

	td.Image = helper.BarChart(v, t.id, string(f.FormatClean([]byte(t.Question))))

	output := bytes.NewBuffer(make([]byte, 0))
	err := timeStatisticsTemplate.Execute(output, td)
	if err != nil {
		log.Printf("time: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (t timeQuestion) ValidateInput(data map[string][]string) error {
	if len(data[t.id]) >= 1 && data[t.id][0] != "" {
		// Valitime Date
		_, err := time.Parse("15:04", data[t.id][0])
		if err == nil {
			return nil
		}
		return fmt.Errorf("time: Can not parse time '%s'", data[t.id][0])
	}
	if t.Required {
		return fmt.Errorf("time: Required, but no input found")
	}
	return nil
}

func (t timeQuestion) GetDatabaseEntry(data map[string][]string) string {
	if len(data[t.id]) >= 1 {
		if data[t.id][0] == "" {
			return ""
		}
		// Valitime Date
		_, err := time.Parse("15:04", data[t.id][0])
		if err == nil {
			return data[t.id][0]
		}
		return strings.Join([]string{"[invalid input]", err.Error()}, " ")
	}
	return ""
}
