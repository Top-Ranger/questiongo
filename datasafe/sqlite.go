//go:build sqlite

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
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/Top-Ranger/questiongo/registry"
)

func init() {
	s := &sqlite{}
	s.newPath = make(chan string)
	s.data = make(chan sqliteResult)
	s.close = make(chan bool)
	s.isClosed = make(chan bool)
	err := registry.RegisterDataSafe(s, "sqlite")
	if err != nil {
		log.Panicln(err)
	}
}

type sqliteResult struct {
	questionnaireID, questionID, data string
}

type sqlite struct {
	db       *sql.DB
	mutex    sync.Mutex
	start    sync.Once
	newPath  chan string
	data     chan sqliteResult
	close    chan bool
	isClosed chan bool
}

func (s *sqlite) IndicateTransactionStart(questionnaireID string) error {
	return nil
}

func (s *sqlite) SaveData(questionnaireID, questionID, data string) error {
	s.data <- sqliteResult{questionnaireID, questionID, data}
	return nil
}

func (s *sqlite) IndicateTransactionEnd(questionnaireID string) error {
	return nil
}

func (s *sqlite) LoadConfig(data []byte) error {
	s.start.Do(func() {
		go s.sqliteWorker()
		log.Println("sqlite: starting worker")
	})
	s.newPath <- strings.TrimSpace(string(data))
	return nil
}

func (s *sqlite) GetData(questionnaireID, questionID string) ([]string, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.db == nil {
		return []string{}, nil
	}

	rows, err := s.db.Query("SELECT data FROM data WHERE questionnaire=? AND question=? ORDER BY id ASC", questionnaireID, questionID)
	defer rows.Close()
	if err != nil {
		return []string{}, err
	}
	result := make([]string, 0)

	for rows.Next() {
		var s string
		err = rows.Scan(&s)
		if err != nil {
			return []string{}, err
		}
		result = append(result, s)
	}

	return result, nil
}

func (s *sqlite) FlushAndClose() {
	select {
	case s.close <- true:
	default:
	}
	_, _ = <-s.isClosed
	return
}

func (s *sqlite) createDB(path string) (*sql.DB, error) {
	// Check if file exists
	newFile := false
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		newFile = true
	} else if err != nil {
		return nil, err
	}

	// Open database
	newDB, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// Create tables if needed
	if newFile {
		tx, err := newDB.Begin()
		if err != nil {
			return nil, err
		}

		_, err = tx.Exec("CREATE TABLE data (questionnaire TEXT, question TEXT, data TEXT, id INTEGER PRIMARY KEY AUTOINCREMENT)")
		if err != nil {
			return nil, err
		}

		err = tx.Commit()
		if err != nil {
			return nil, err
		}
	}
	return newDB, nil
}

func (s *sqlite) sqliteWorker() {
	buffer := make([]sqliteResult, 0, 10)
	tick := time.NewTicker(5 * time.Second)
	closeWorker := false
	for {
		select {
		case <-s.close:
			if !closeWorker {
				log.Printf("sqlite: starting flush")
				closeWorker = true
			}
		case p := <-s.newPath:
			if closeWorker {
				log.Printf("sqlite: Ignoring new path %s since close has been called.", p)
				continue
			}
			func() {
				s.mutex.Lock()
				defer s.mutex.Unlock()
				if s.db != nil {
					s.db.Close()
					s.db = nil
				}
				var err error
				s.db, err = s.createDB(p)
				if err != nil {
					log.Printf("sqlite: Can not create %s: %s", p, err.Error())
				} else {
					buffer = make([]sqliteResult, 0, 10)
				}
			}()
		case d := <-s.data:
			if s.db == nil {
				fmt.Printf("sqlite: Not saving result - worker not running (%v)", d)
				continue
			}
			buffer = append(buffer, d)
		case <-tick.C:
			func() {
				s.mutex.Lock()
				defer s.mutex.Unlock()
				if s.db == nil || len(buffer) == 0 {
					return
				}
				tx, err := s.db.Begin()
				if err != nil {
					log.Printf("sqlite: Can not create transaction: %s", err.Error())
					s.db = nil
					return
				}
				for i := range buffer {
					_, err = tx.Exec("INSERT INTO data (questionnaire, question, data) VALUES (?, ?, ?)", buffer[i].questionnaireID, buffer[i].questionID, buffer[i].data)
					if err != nil {
						log.Printf("sqlite: Can not insert into database: %s", err.Error())
						tx.Rollback()
						s.db = nil
						return
					}
				}
				err = tx.Commit()
				if err != nil {
					log.Printf("sqlite: Can not commit transaction: %s", err.Error())
					s.db = nil
					return
				}
				newLen := len(buffer) * 2
				if newLen < 10 {
					newLen = 10
				}
				buffer = make([]sqliteResult, 0, newLen)
			}()
			if closeWorker {
				log.Printf("sqlite: flushed")
				s.db.Close()
				s.isClosed <- true
				close(s.isClosed)
				return
			}
		}
	}
}
