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

package datasafe

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Top-Ranger/questiongo/registry"
)

func init() {
	fa := &fileAppend{}
	fa.newPath = make(chan string)
	fa.data = make(chan fileAppendResult)
	fa.close = make(chan bool)
	fa.isClosed = make(chan bool)
	err := registry.RegisterDataSafe(fa, "fileappend")
	if err != nil {
		log.Panicln(err)
	}
}

type fileAppendResult struct {
	questionnaireID, questionID, data string
}

type fileAppendResultBuffer []fileAppendResult

func (f fileAppendResultBuffer) Len() int {
	return len(f)
}

func (f fileAppendResultBuffer) Less(i, j int) bool {
	if f[i].questionnaireID < f[j].questionnaireID {
		return true
	} else if f[i].questionnaireID > f[j].questionnaireID {
		return false
	}
	return f[i].questionID < f[j].questionID
}

func (f fileAppendResultBuffer) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

type fileAppend struct {
	path     string
	mutex    sync.Mutex
	start    sync.Once
	newPath  chan string
	data     chan fileAppendResult
	close    chan bool
	isClosed chan bool
}

func (fa *fileAppend) IndicateTransactionStart(questionnaireID string) error {
	return nil
}

func (fa *fileAppend) SaveData(questionnaireID, questionID, data string) error {
	fa.data <- fileAppendResult{questionnaireID, questionID, data}
	return nil
}

func (fa *fileAppend) IndicateTransactionEnd(questionnaireID string) error {
	return nil
}

func (fa *fileAppend) LoadConfig(data []byte) error {
	fa.start.Do(func() {
		go fa.fileappendWorker()
		log.Println("FileAppend: starting worker")
	})
	fa.newPath <- strings.TrimSpace(string(data))
	return nil
}

func (fa *fileAppend) GetData(questionnaireID, questionID string) ([]string, error) {
	fa.mutex.Lock()
	defer fa.mutex.Unlock()
	b, err := os.ReadFile(filepath.Join(fa.path, questionnaireID, questionID))
	if os.IsNotExist(err) {
		// No data was written - thats ok
		return []string{}, nil
	} else if err != nil {
		return []string{}, err
	}
	s := strings.TrimSuffix(string(b), "\n")
	split := strings.Split(s, "\n")
	for i := range split {
		split[i] = strings.ReplaceAll(split[i], "󰀕", "\n")
	}
	return split, nil
}

func (fa *fileAppend) FlushAndClose() {
	select {
	case fa.close <- true:
	default:
	}
	<-fa.isClosed
}

func (fa *fileAppend) fileappendWorker() {
	buffer := fileAppendResultBuffer(make([]fileAppendResult, 0, 10))
	tick := time.NewTicker(5 * time.Second)
	running := false
	closeWorker := false
	for {
		select {
		case <-fa.close:
			if !closeWorker {
				log.Printf("FileAppend: starting flush")
				closeWorker = true
			}
		case p := <-fa.newPath:
			if closeWorker {
				log.Printf("FileAppend: Ignoring new path %s since close has been called.", p)
				continue
			}
			func() {
				fa.mutex.Lock()
				defer fa.mutex.Unlock()
				fa.path = p
				err := os.MkdirAll(fa.path, os.ModePerm)
				if err != nil {
					log.Printf("FileAppend: Can not create %s: %s", p, err.Error())
				} else {
					running = true
					buffer = make([]fileAppendResult, 0, 10)
				}
			}()
		case d := <-fa.data:
			if !running {
				fmt.Printf("FileAppend: Not saving result - worker not running (%v)", d)
				continue
			}
			buffer = append(buffer, d)
		case <-tick.C:
			func() {
				fa.mutex.Lock()
				defer fa.mutex.Unlock()
				sort.Stable(buffer) // We need to preserve the order of the answers
				for i := 0; i < len(buffer); i++ {
					err := os.MkdirAll(filepath.Join(fa.path, buffer[i].questionnaireID), os.ModePerm)
					if err != nil {
						log.Printf("FileAppend: Can not create %s: %s", filepath.Join(fa.path, buffer[i].questionnaireID), err.Error())
						running = false
						return
					}

					func() {
						path := filepath.Join(fa.path, buffer[i].questionnaireID, buffer[i].questionID)
						f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
						if err != nil {
							log.Printf("FileAppend: Can not create %s: %s", path, err.Error())
							running = false
							return
						}
						defer f.Close()

						write := true
						for write {
							write = false
							s := strings.ReplaceAll(buffer[i].data, "󰀕", "") // Remove invalid characters. This are not allowed to be used anyway
							s = strings.ReplaceAll(s, "\n", "󰀕")
							_, err = f.Write([]byte(s))
							if err != nil {
								log.Printf("FileAppend: Can not write to %s: %s", path, err.Error())
								running = false
								return
							}
							_, err = f.Write([]byte("\n"))
							if err != nil {
								log.Printf("FileAppend: Can not write to %s: %s", path, err.Error())
								running = false
								return
							}
							for i < len(buffer)-1 && buffer[i+1].questionnaireID == buffer[i].questionnaireID && buffer[i+1].questionID == buffer[i].questionID {
								write = true
								i++
							}
						}
					}()
				}
				newLen := len(buffer) * 2
				if newLen < 10 {
					newLen = 10
				}
				buffer = make([]fileAppendResult, 0, newLen)
			}()
			if closeWorker {
				log.Printf("FileAppend: flushed")
				fa.isClosed <- true
				close(fa.isClosed)
				return
			}
		}
	}
}
