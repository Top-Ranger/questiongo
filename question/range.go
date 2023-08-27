// SPDX-License-Identifier: Apache-2.0
// Copyright 2020,2022,2023 Marcus Soll
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
	"strconv"
	"strings"

	"github.com/Top-Ranger/questiongo/helper"
	"github.com/Top-Ranger/questiongo/registry"
)

func init() {
	err := registry.RegisterQuestionType(FactoryRange, "range")
	if err != nil {
		panic(err)
	}
}

// FactoryRange is the factory for range questions.
func FactoryRange(data []byte, id string, language string) (registry.Question, error) {
	var r rangeQuestion
	err := json.Unmarshal(data, &r)
	if err != nil {
		return nil, err
	}
	r.id = id

	if r.Max < r.Min {
		return nil, fmt.Errorf("range: max (%d) must be larger than min (%d) (%s)", r.Max, r.Min, id)
	}

	if r.Step < 1 {
		return nil, fmt.Errorf("range: step (%d) must be at least 1", r.Step)
	}

	if r.Step > r.Max-r.Min {
		return nil, fmt.Errorf("range: step (%d) must be smaller than the range (%d) (%s)", r.Step, r.Max-r.Min, id)
	}

	if r.Start > r.Max || r.Start < r.Min {
		return nil, fmt.Errorf("range: start (%d) must be between min (%d) and max (%d) (%s)", r.Start, r.Min, r.Max, id)
	}

	_, ok := registry.GetFormatType(r.Format)
	if !ok {
		return nil, fmt.Errorf("range: Unknown format type %s (%s)", r.Format, id)
	}

	return &r, nil
}

var rangeTemplate = template.Must(template.New("rangeTemplate").Parse(`<label for="{{.QID}}">{{.Question}}</label>
{{if .ShowScale}}
<div style="min-height: 2em; max-width: 700px;">
<div style="float: left;">{{.ScaleStart}}</div>
<div style="float: right;">{{.ScaleEnd}}</div>
</div>
{{end}}
<input style="width: 100%; max-width: 700px; text-align: center;" type="range" id="{{.QID}}" name="{{.QID}}" min="{{.Min}}" max="{{.Max}}" step="{{.Step}}" value="{{.Start}}" {{if .ShowValue}}oninput="document.getElementById('{{.QID}}_output').value = this.value;"{{end}}>
{{if .ShowValue}}
<div style="text-align: center; max-width: 700px;">
<output id="{{.QID}}_output" style="display: inline" for="{{.QID}}">{{.Start}}</output>
</div>
{{end}}
<br>
`))

var rangeStatisticsTemplate = template.Must(template.New("rangeStatisticTemplate").Parse(`{{.Question}}<br>
<table>
<thead>
<tr>
<th>Value</th>
<th>Number</th>
<th>Percent</th>
</tr>
</thead>
{{range $i, $e := .Data }}
<tr>
<td>{{$e.Value}}</td>
<td>{{$e.Number}}</td>
<td>{{printf "%.2f" $e.Percent}}</td>
</tr>
{{end}}
<tr>
<td class="th-cell">[average]</td>
<td>{{printf "%.2f" .Average}}</td>
</tr>
<tr>
<td class="th-cell">[number answer]</td>
<td>{{.Count}}</td>
</tr>
<tr>
<td class="th-cell">[invalid input]</td>
<td>{{.Invalid}}</td>
</tr>
</tbody>
</table>
<br>
{{.Image}}
`))

type rangeTemplateStruct struct {
	Question   template.HTML
	QID        string
	Min        int
	Max        int
	Step       int
	Start      int
	ShowValue  bool
	ShowScale  bool
	ScaleStart string
	ScaleEnd   string
}

type rangeStatisticTemplateStructInner struct {
	Value   int
	Number  int
	Percent float64
}

type rangeStatisticTemplateStruct struct {
	Question template.HTML
	Data     []rangeStatisticTemplateStructInner
	Average  float64
	Count    int
	Invalid  int
	Image    template.HTML
}
type rangeStatisticTemplateStructInnerSort []rangeStatisticTemplateStructInner

func (r rangeStatisticTemplateStructInnerSort) Len() int {
	return len(r)
}

func (r rangeStatisticTemplateStructInnerSort) Less(i, j int) bool {
	return r[i].Value < r[j].Value
}

func (r rangeStatisticTemplateStructInnerSort) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

type rangeQuestion struct {
	Format     string
	Question   string
	Min        int
	Max        int
	Step       int
	Start      int
	ShowValue  bool
	ShowScale  bool
	ScaleStart string
	ScaleEnd   string

	id string
}

func (r rangeQuestion) GetID() string {
	return r.id
}

func (r rangeQuestion) GetHTML() template.HTML {
	f, _ := registry.GetFormatType(r.Format)

	td := rangeTemplateStruct{
		Question:   f.Format([]byte(r.Question)),
		QID:        r.id,
		Min:        r.Min,
		Max:        r.Max,
		Step:       r.Step,
		Start:      r.Start,
		ShowValue:  r.ShowValue,
		ShowScale:  r.ShowScale,
		ScaleStart: r.ScaleStart,
		ScaleEnd:   r.ScaleEnd,
	}

	output := bytes.NewBuffer(make([]byte, 0))
	err := rangeTemplate.Execute(output, td)
	if err != nil {
		log.Printf("range: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (r rangeQuestion) GetStatisticsHeader() []string {
	return []string{r.id}
}

func (r rangeQuestion) GetStatistics(data []string) [][]string {
	result := make([][]string, len(data))
	for i := range data {
		value, err := strconv.Atoi(data[i])

		if err != nil {
			result[i] = []string{strings.Join([]string{"[invalid input]", err.Error()}, " ")}
		} else if value < r.Min || value > r.Max {
			result[i] = []string{fmt.Sprintf("[invalid input] value %d out of range", value)}
		} else {
			result[i] = []string{data[i]}
		}
	}
	return result
}

func (r rangeQuestion) GetStatisticsDisplay(data []string) template.HTML {
	f, _ := registry.GetFormatType(r.Format)

	td := rangeStatisticTemplateStruct{
		Question: f.Format([]byte(r.Question)),
		Average:  0.0,
		Count:    0,
		Invalid:  0,
	}

	answer := make(map[int]int)

	for i := range data {
		value, err := strconv.Atoi(data[i])
		if err != nil {
			td.Invalid++
		} else {
			td.Count++
			answer[value]++
			td.Average += float64(value)
		}
	}

	for k := range answer {
		td.Data = append(td.Data, rangeStatisticTemplateStructInner{Value: k, Number: answer[k], Percent: float64(answer[k]) / float64(td.Count)})
	}

	sort.Sort(rangeStatisticTemplateStructInnerSort(td.Data))

	v := make([]helper.ChartValue, len(td.Data))

	for i := range td.Data {
		v[i].Label = strconv.Itoa(td.Data[i].Value)
		v[i].Value = float64(td.Data[i].Number)
	}

	td.Image = helper.BarChart(v, r.id, string(f.FormatClean([]byte(r.Question))))

	td.Average /= float64(td.Count)

	output := bytes.NewBuffer(make([]byte, 0))
	err := rangeStatisticsTemplate.Execute(output, td)
	if err != nil {
		log.Printf("range: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (r rangeQuestion) ValidateInput(data map[string][]string) error {
	if len(data[r.id]) == 0 || data[r.id][0] == "" {
		return fmt.Errorf("range (%s): No input found", r.id)
	}
	value, err := strconv.Atoi(data[r.id][0])
	if err != nil {
		return fmt.Errorf("range: Input '%s' malformed (%s)", data[r.id][0], err.Error())
	}
	if value == r.Start {
		return nil
	}
	i := r.Min
	for i <= r.Max {
		if i == value {
			return nil
		}
		i += r.Step
	}
	return fmt.Errorf("range: Input '%d' not in range", value)
}

func (r rangeQuestion) IgnoreRecord(data map[string][]string) bool {
	return false
}

func (r rangeQuestion) GetDatabaseEntry(data map[string][]string) string {
	if len(data[r.id]) >= 1 {
		return data[r.id][0]
	}
	return ""
}
