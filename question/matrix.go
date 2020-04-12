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
	err := registry.RegisterQuestionType(FactoryMatrix, "matrix")
	if err != nil {
		panic(err)
	}
}

// FactoryMatrix is the factory for matrix questions.
func FactoryMatrix(data []byte, id string) (registry.Question, error) {
	var m matrix
	err := json.Unmarshal(data, &m)
	if err != nil {
		return nil, err
	}
	m.id = id

	// Sanity checks
	testID := make(map[string]bool)
	for i := range m.Answers {
		if len(m.Answers[i]) != 2 {
			return nil, fmt.Errorf("matrix: Answer %d must have exactly 2 values (id, text) (%s)", i, id)
		}
		if testID[m.Answers[i][0]] {
			return nil, fmt.Errorf("matrix: ID %s found twice (%s)", m.Answers[i][0], id)
		}
		testID[m.Answers[i][0]] = true
	}

	testID = make(map[string]bool)
	for i := range m.Questions {
		if len(m.Questions[i]) != 2 {
			return nil, fmt.Errorf("matrix: Answer %d must have exactly 2 values (id, text) (%s)", i, id)
		}
		if testID[m.Questions[i][0]] {
			return nil, fmt.Errorf("matrix: ID %s found twice (%s)", m.Questions[i][0], id)
		}
		testID[m.Questions[i][0]] = true
	}

	_, ok := registry.GetFormatType(m.Format)
	if !ok {
		return nil, fmt.Errorf("matrix: Unknown format type %s (%s)", m.Format, id)
	}

	return &m, nil
}

var matrixTemplate = template.Must(template.New("matrixTemplate").Parse(`{{.Title}}<br>
<table>
<thead>
<tr>
<th></th>
{{range $i, $e := .Header }}
<th class="centre">{{$e}}</th>
{{end}}
</tr>
</thead>
<tbody>
{{range $i, $e := .Data }}
<tr>
<td><strong>{{$e.Question}}</strong></td>
{{range $I, $E := $.Answer }}
<td class="centre" title="{{$e.Question}} - {{index $E 1}}"><input title="{{$e.Question}} - {{index $E 1}}" type="radio" name="{{$.GID}}_{{$e.QID}}" value="{{index $E 0}}" {{if $.Required}} required {{end}}></td>
{{end}}
</tr>
{{end}}
</tbody>
</table>
`))

var matrixStatisticsTemplate = template.Must(template.New("matrixStatisticTemplate").Parse(`{{.Title}}<br>
<table>
<thead>
<tr>
<th>Question</th>
{{range $i, $e := .Header }}
<th>{{$e}}</th>
{{end}}
</tr>
</thead>
{{range $i, $e := .Data }}
<tr>
<td><strong>{{$e.Question}}</strong></td>
{{range $I, $E := $e.Result }}
<td>{{printf "%.2f" $E}}</td>
{{end}}
</tr>
{{end}}
</tbody>
</table>
{{range $i, $e := .Images }}
<br>
{{$e}}
{{end}}
`))

type matrixTemplateStructInner struct {
	Question template.HTML
	QID      string
}

type matrixTemplateStruct struct {
	Title    template.HTML
	Required bool
	Header   []template.HTML
	Data     []matrixTemplateStructInner
	Answer   [][]string
	GID      string
}

type matrixStatisticsTemplateStructInner struct {
	Question template.HTML
	Result   []float64
}

type matrixStatisticTemplateStruct struct {
	Title  template.HTML
	Header []template.HTML
	Data   []matrixStatisticsTemplateStructInner
	Images []template.HTML
}

type matrix struct {
	Random    bool
	Required  bool
	Format    string
	Title     string
	Answers   [][]string
	Questions [][]string

	id string
}

func (m matrix) GetID() string {
	return m.id
}

func (m matrix) GetHTML() template.HTML {
	f, _ := registry.GetFormatType(m.Format)
	td := matrixTemplateStruct{
		Title:    f.Format([]byte(m.Title)),
		Required: m.Required,
		Header:   make([]template.HTML, len(m.Answers)),
		Data:     make([]matrixTemplateStructInner, 0, len(m.Questions)),
		Answer:   make([][]string, len(m.Answers)),
		GID:      m.id,
	}
	for i := range m.Questions {
		mts := matrixTemplateStructInner{
			QID:      m.Questions[i][0],
			Question: f.FormatClean([]byte(m.Questions[i][1])),
		}
		td.Data = append(td.Data, mts)
	}
	for i := range m.Answers {
		td.Header[i] = f.Format([]byte(m.Answers[i][1]))
		td.Answer[i] = []string{m.Answers[i][0], m.Answers[i][1]}
	}

	if m.Random {
		rand.Shuffle(len(td.Data), func(i, j int) {
			td.Data[i], td.Data[j] = td.Data[j], td.Data[i]
		})
	}

	output := bytes.NewBuffer(make([]byte, 0))
	err := matrixTemplate.Execute(output, td)
	if err != nil {
		log.Printf("matrix: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (m matrix) GetStatisticsHeader() []string {
	header := make([]string, len(m.Questions))
	for i := range m.Questions {
		header[i] = fmt.Sprintf("%s_%s", m.id, m.Questions[i][0])
	}
	return header
}

func (m matrix) GetStatistics(data []string) [][]string {
	result := make([][]string, len(data))
	for d := range data {
		r := make([]string, len(m.Questions))
		errorState := true
		if !strings.HasPrefix(data[d], "ERROR") {
			rarray := make([]string, len(m.Answers))
			err := json.Unmarshal([]byte(data[d]), &rarray)
			if err == nil && len(rarray) == len(r) {
				errorState = false
				for i := range rarray {
					for j := range m.Answers {
						if rarray[i] == m.Answers[j][0] {
							r[i] = m.Answers[j][0]
							break
						}
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

func (m matrix) GetStatisticsDisplay(data []string) template.HTML {
	count := 0
	countAnswer := make([][]int, len(m.Questions))
	for i := range m.Questions {
		countAnswer[i] = make([]int, len(m.Answers)+1)
	}

	for d := range data {
		rarray := make([]string, len(m.Questions))
		err := json.Unmarshal([]byte(data[d]), &rarray)
		if err != nil {
			continue
		}
		if len(rarray) != len(m.Questions) {
			fmt.Println(99999)
			continue
		}
		count++

		for i := range m.Questions {
			found := false
			for j := range m.Answers {
				if rarray[i] == m.Answers[j][0] {
					countAnswer[i][j]++
					found = true
					break
				}
			}
			if !found {
				countAnswer[i][len(m.Answers)]++
			}
		}
	}

	f, _ := registry.GetFormatType(m.Format)
	td := matrixStatisticTemplateStruct{
		Title:  f.Format([]byte(m.Title)),
		Header: make([]template.HTML, len(m.Answers)+1),
		Data:   make([]matrixStatisticsTemplateStructInner, 0, len(m.Questions)),
		Images: make([]template.HTML, 0, len(m.Questions)),
	}
	for i := range m.Answers {
		td.Header[i] = f.FormatClean([]byte(m.Answers[i][1]))
	}
	td.Header[len(m.Answers)] = "[no answer]"

	for i := range m.Questions {
		v := make([]helper.ChartValue, len(m.Answers)+1)
		question := f.FormatClean([]byte(m.Questions[i][1]))
		inner := matrixStatisticsTemplateStructInner{
			Question: question,
			Result:   make([]float64, len(m.Answers)+1),
		}
		for j := range m.Answers {
			inner.Result[j] = float64(countAnswer[i][j]) / float64(count)
			v[j].Label = string(td.Header[j])
			v[j].Value = float64(countAnswer[i][j])
		}
		inner.Result[len(m.Answers)] = float64(countAnswer[i][len(m.Answers)]) / float64(count)
		v[len(m.Answers)].Label = "[no answer]"
		v[len(m.Answers)].Value = float64(countAnswer[i][len(m.Answers)])

		td.Images = append(td.Images, helper.PieChart(v, fmt.Sprintf("%s_%s", m.id, string(helper.SanitiseStringClean(m.Questions[i][0]))), string(question)))

		td.Data = append(td.Data, inner)
	}
	output := bytes.NewBuffer(make([]byte, 0))
	err := matrixStatisticsTemplate.Execute(output, td)
	if err != nil {
		log.Printf("matrix: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}

func (m matrix) GetDatabaseEntry(data map[string][]string) string {
	result := make([]string, len(m.Questions))
	for i := range m.Questions {
		r, ok := data[fmt.Sprintf("%s_%s", m.id, m.Questions[i][0])]
		if ok && len(r) >= 1 {
			result[i] = r[0]
		} else {
			result[i] = ""
		}
	}
	b, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf("ERROR: %s", err.Error())
	}
	return string(b)
}
