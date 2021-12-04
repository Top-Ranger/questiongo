//go:build mysql

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

package datasafe

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"

	_ "github.com/go-sql-driver/mysql"

	"github.com/Top-Ranger/questiongo/registry"
)

func init() {
	m := &mySQL{}
	m.txCacheMutex = new(sync.Mutex)
	err := registry.RegisterDataSafe(m, "MySQL")
	if err != nil {
		log.Panicln(err)
	}
}

// MySQLMaxLengthID is the maximum supported id length
const MySQLMaxLengthID = 500

// ErrMySQLUnknownID is returned when the id of the requested item is too long
var ErrMySQLIDtooLong = errors.New("mysql: id is too long")

// ErrMySQLNotConfigured is returned when the database is used before it is configured
var ErrMySQLNotConfigured = errors.New("mysql: usage before configuration is used")

type mySQL struct {
	dsn          string
	db           *sql.DB
	txCache      map[string]*sql.Tx
	txCacheMutex *sync.Mutex
}

func (m *mySQL) IndicateTransactionStart(questionnaireID string) error {
	if m.db == nil {
		return ErrMySQLNotConfigured
	}

	if len(questionnaireID) > MySQLMaxLengthID {
		return ErrMySQLIDtooLong
	}

	m.txCacheMutex.Lock()
	defer m.txCacheMutex.Unlock()

	var err error

	tx := m.txCache[questionnaireID]
	if tx == nil {
		tx, err = m.db.Begin()
		if err != nil {
			return err
		}
		m.txCache[questionnaireID] = tx
	}

	return nil
}

func (m *mySQL) SaveData(questionnaireID, questionID, data string) error {
	if m.db == nil {
		return ErrMySQLNotConfigured
	}

	if len(questionnaireID) > MySQLMaxLengthID {
		return ErrMySQLIDtooLong
	}

	if len(questionID) > MySQLMaxLengthID {
		return ErrMySQLIDtooLong
	}

	m.txCacheMutex.Lock()
	tx := m.txCache[questionnaireID]
	m.txCacheMutex.Unlock()

	if tx != nil {
		_, err := tx.Exec("INSERT INTO data (questionnaire, question, data) VALUES (?,?,?)", questionnaireID, questionID, data)
		return err
	}

	_, err := m.db.Exec("INSERT INTO data (questionnaire, question, data) VALUES (?,?,?)", questionnaireID, questionID, data)

	return err
}

func (m *mySQL) IndicateTransactionEnd(questionnaireID string) error {
	if m.db == nil {
		return ErrMySQLNotConfigured
	}

	if len(questionnaireID) > MySQLMaxLengthID {
		return ErrMySQLIDtooLong
	}

	m.txCacheMutex.Lock()
	defer m.txCacheMutex.Unlock()

	var err error

	tx := m.txCache[questionnaireID]

	if tx != nil {
		err = tx.Commit()
		m.txCache[questionnaireID] = nil
		if err != nil {
			tx.Rollback()
			return err
		}
	}

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

func (m *mySQL) GetData(questionnaireID, questionID string) ([]string, error) {
	if m.db == nil {
		return nil, ErrMySQLNotConfigured
	}

	if len(questionnaireID) > MySQLMaxLengthID {
		return nil, ErrMySQLIDtooLong
	}

	if len(questionID) > MySQLMaxLengthID {
		return nil, ErrMySQLIDtooLong
	}

	rows, err := m.db.Query("SELECT data FROM data WHERE questionnaire=? AND question=? ORDER BY id ASC", questionnaireID, questionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	data := make([]string, 0)

	for rows.Next() {
		var s string
		err = rows.Scan(&s)
		if err != nil {
			return nil, err
		}
		data = append(data, s)
	}

	return data, nil
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
