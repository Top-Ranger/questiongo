// SPDX-License-Identifier: Apache-2.0
// Copyright 2020,2021 Marcus Soll
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

package helper

import (
	"bytes"
	"html/template"
	"log"
)

var stacked100Template = template.Must(template.New("chartTemplate").Parse(`
<div class="chart" style="width: {{.Width}}vw;">
	<canvas id="{{.ID}}"></canvas>
</div>
<script>
var ctx = document.getElementById('{{.ID}}').getContext('2d');
new Chart(ctx, {
	type: {{.Type}},
	data: {
		datasets: [
			{{range $i, $e := .NumberValues }}	
			{
				label: {{index $.LabelValues $i}},
				data: [
					{{range $I, $E := $.Data }}
						{{index $E $i}},
					{{end}}
				],
				backgroundColor: {{index $.Colour $i}},
			},
			{{end}}
		],
		labels: [
			{{range $i, $e := .LabelBars }}
			{{$e}},
			{{end}}
		],

	},
	options: {
		indexAxis: "y",
		plugins: {
			{{if .Title}}
			title: {
				display: true,
				text: {{.Title}}
			},
			{{end}}
			stacked100: { enable: true },
		},
	}
});
</script>
`))

type stacked100TemplateStruct struct {
	Data         [][]int
	Colour       []string
	ID           string
	Type         string
	LabelBars    []string
	LabelValues  []string
	NumberValues []int
	Title        string
	Width        int
}

// BarChart returns a save HTML fragment of the data as a 100% stacked bar chart.
// v is interpreted as v[bar][value]. Missing labels wikk be filled with empty labels.
// User must embed chart.js and chartjs-plugin-stacked100.
func Stacked100Chart(v [][]int, id string, labelBars []string, LabelValues []string, title string) template.HTML {
	for len(v) > len(labelBars) {
		labelBars = append(labelBars, "")
	}
	max := 0
	for i := range v {
		if len(v[i]) > max {
			max = len(v[i])
		}
	}
	for i := range v {
		for max > len(v[i]) {
			v[i] = append(v[i], 0)
		}
	}
	for max > len(LabelValues) {
		LabelValues = append(LabelValues, "")
	}

	td := stacked100TemplateStruct{
		Data:         v,
		Colour:       getColours(len(LabelValues)),
		ID:           id,
		Type:         "bar",
		LabelBars:    labelBars,
		LabelValues:  LabelValues,
		NumberValues: make([]int, max),
		Title:        title,
		Width:        len(labelBars) * 5,
	}
	if td.Width > 80 {
		td.Width = 80
	}
	if td.Width < 15 {
		td.Width = 15
	}
	output := bytes.NewBuffer(make([]byte, 0))
	err := stacked100Template.Execute(output, td)
	if err != nil {
		log.Printf("bar chart: Error executing template (%s)", err.Error())
	}
	return template.HTML(output.Bytes())
}
