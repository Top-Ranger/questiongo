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

package format

import (
	"fmt"
	"html/template"

	"github.com/Top-Ranger/questiongo/registry"
)

func init() {
	registry.RegisterFormatType(Plain{}, "plain")
}

// Plain is a formatting which wraps the plain input into HTML.
type Plain struct{}

// Format returns a save html version of the input.
func (p Plain) Format(b []byte) template.HTML {
	s := template.HTMLEscaper(string(b))
	return template.HTML(fmt.Sprintf("<p>%s</p>", s))
}

// FormatClean returns a save html version of the input.
func (p Plain) FormatClean(b []byte) template.HTML {
	return template.HTML(template.HTMLEscaper(string(b)))
}
