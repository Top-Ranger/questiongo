//go:build mysql

// SPDX-License-Identifier: Apache-2.0
// Copyright 2021,2022 Marcus Soll
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
	"errors"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"

	"github.com/Top-Ranger/questiongo/registry"
)

func init() {
	m := &mySQL{}
	err := registry.RegisterDataSafe(m, "MySQL")
	if err != nil {
		log.Panicln(err)
	}
}

// MySQLMaxLengthID is the maximum supported id length
const MySQLMaxLengthID = 150

// ErrMySQLUnknownID is returned when the id of the requested item is too long
var ErrMySQLIDtooLong = errors.New("mysql: id is too long")

// ErrMySQLNotConfigured is returned when the database is used before it is configured
var ErrMySQLNotConfigured = errors.New("mysql: usage before configuration is used")

type mySQL struct {
	dsn string
	db  *sql.DB
}

func (m *mySQL) SaveData(questionnaireID string, questionID, data []string) error {
	if m.db == nil {
		return ErrMySQLNotConfigured
	}

	if len(questionnaireID) > MySQLMaxLengthID {
		return ErrMySQLIDtooLong
	}

	if len(questionID) != len(data) {
		return fmt.Errorf("mysql: len(questionID)=%d does not match len(data)=%d", len(questionID), len(data))
	}

	for i := range questionID {
		if len(questionID[i]) > MySQLMaxLengthID {
			return ErrMySQLIDtooLong
		}
	}

	tx, err := m.db.Begin()
	if err != nil {
		return err
	}

	successful := false

	defer func() {
		if !successful {
			err := tx.Rollback()
			if err != nil {
				log.Printf("mysql: can not rollback transaction: %s", err.Error())
			}
		}
	}()

	for i := range questionID {
		_, err := tx.Exec("INSERT INTO data (questionnaire, question, data) VALUES (?,?,?)", questionnaireID, questionID[i], data[i])
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	successful = true
	return nil
}

func (m *mySQL) LoadConfig(data []byte) error {
	m.dsn = string(data)
	db, err := sql.Open("mysql", m.dsn)
	if err != nil {
		return fmt.Errorf("mysql: can not open '%s': %w", m.dsn, err)
	}
	m.db = db
	return nil
}

func (m *mySQL) GetData(questionnaireID string, questionID []string) ([][]string, error) {
	if m.db == nil {
		return nil, ErrMySQLNotConfigured
	}

	if len(questionnaireID) > MySQLMaxLengthID {
		return nil, ErrMySQLIDtooLong
	}

	if len(questionID) > MySQLMaxLengthID {
		return nil, ErrMySQLIDtooLong
	}

	tx, err := m.db.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Commit()

	result := make([][]string, len(questionID))

	for i := range questionID {
		rows, err := tx.Query("SELECT data FROM data WHERE questionnaire=? AND question=? ORDER BY id ASC", questionnaireID, questionID[i])
		if err != nil {
			return nil, err
		}

		data := make([]string, 0)

		for rows.Next() {
			var s string
			err = rows.Scan(&s)
			if err != nil {
				rows.Close()
				return nil, err
			}
			data = append(data, s)
		}
		result[i] = data
		rows.Close()
	}

	return result, nil
}

func (m *mySQL) FlushAndClose() {
	if m.db == nil {
		return
	}

	err := m.db.Close()
	if err != nil {
		log.Printf("mysql: error closing db: %s", err.Error())
	}
}
