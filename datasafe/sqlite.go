//go:build sqlite

// SPDX-License-Identifier: Apache-2.0
// Copyright 2021,2022,2026 Marcus Soll
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
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/Top-Ranger/questiongo/registry"
)

func init() {
	m := &sqlite{}
	err := registry.RegisterDataSafe(m, "SQLite3")
	if err != nil {
		log.Panicln(err)
	}
	err = registry.RegisterDataSafe(m, "SQLite")
	if err != nil {
		log.Panicln(err)
	}
}

// SQLiteMaxLengthID is the maximum supported id length
const SQLiteMaxLengthID = 150

// ErrSQLiteIDtooLong is returned when the id of the requested item is too long
var ErrSQLiteIDtooLong = errors.New("mysql: id is too long")

// ErrSQliteNotConfigured is returned when the database is used before it is configured
var ErrSQliteNotConfigured = errors.New("sqlite: usage before configuration is used")

type sqlite struct {
	path string
	db   *sql.DB
}

func (m *sqlite) SaveData(questionnaireID string, questionID, data []string) error {
	if m.db == nil {
		return ErrSQliteNotConfigured
	}

	if len(questionnaireID) > SQLiteMaxLengthID {
		return ErrSQLiteIDtooLong
	}

	if len(questionID) != len(data) {
		return fmt.Errorf("sqlite: len(questionID)=%d does not match len(data)=%d", len(questionID), len(data))
	}

	for i := range questionID {
		if len(questionID[i]) > SQLiteMaxLengthID {
			return ErrSQLiteIDtooLong
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
				log.Printf("sqlite: can not rollback transaction: %s", err.Error())
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

func (m *sqlite) LoadConfig(data []byte) error {
	m.path = string(data)

	exists := true
	if _, err := os.Stat(m.path); errors.Is(err, os.ErrNotExist) {
		exists = false
	}

	db, err := sql.Open("sqlite3", m.path)
	if err != nil {
		return fmt.Errorf("sqlite: can not open '%s': %w", m.path, err)
	}
	db.SetConnMaxLifetime(time.Minute * 1)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	m.db = db

	if !exists {
		_, err = db.Exec("PRAGMA journal_mode=WAL;")
		if err != nil {
			return fmt.Errorf("sqlite: can not set journal to WAL: %w", err)
		}
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			return fmt.Errorf("sqlite: can not start commit to create db: %w", err)
		}
		_, err = tx.Exec("CREATE TABLE meta (key string, data string, PRIMARY KEY(key));")
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("sqlite: can not create table meta: %w", err)
		}
		_, err = tx.Exec("INSERT INTO meta VALUES (?, ?);", "version", "1")
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("sqlite: can not insert version: %w", err)
		}
		_, err = tx.Exec("CREATE TABLE data (id INTEGER PRIMARY KEY AUTOINCREMENT, questionnaire VARCHAR(200) NOT NULL, question VARCHAR(200) NOT NULL, data LONGTEXT NOT NULL);")
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("sqlite: can not create table data: %w", err)
		}
		_, err = tx.Exec("CREATE INDEX qda ON data (questionnaire,question);")
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("sqlite: can not create index qda on data: %w", err)
		}

		tx.Commit()
	}

	return nil
}

func (m *sqlite) GetData(questionnaireID string, questionID []string) ([][]string, error) {
	if m.db == nil {
		return nil, ErrSQliteNotConfigured
	}

	if len(questionnaireID) > SQLiteMaxLengthID {
		return nil, ErrSQLiteIDtooLong
	}

	if len(questionID) > SQLiteMaxLengthID {
		return nil, ErrSQLiteIDtooLong
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

func (m *sqlite) FlushAndClose() {
	if m.db == nil {
		return
	}

	err := m.db.Close()
	if err != nil {
		log.Printf("sqlite: error closing db: %s", err.Error())
	}
}
