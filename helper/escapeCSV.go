// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Marcus Soll
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
	"fmt"
	"strings"
)

// EscapeCSVLine escapes a CSV line (as a []string) so it can be considered save with spreadsheet programs.
// Escaping is according to https://owasp.org/www-community/attacks/CSV_Injection
func EscapeCSVLine(input []string) []string {
	for i := range input {
		input[i] = strings.TrimLeft(input[i], "\t\r")
		if len(input[i]) == 0 {
			continue
		}

		switch []rune(input[i])[0] {
		case '=', '+', '-', '@':
			input[i] = fmt.Sprintf("'%s", input[i])
		}
	}
	return input
}
