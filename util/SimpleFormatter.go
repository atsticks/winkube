// Copyright 2019 Anatole Tresch
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"fmt"
	log "github.com/sirupsen/logrus"
)

/*
 * Implementation of a simple log formatter.
 */
type PlainFormatter struct {
	TimestampFormat string
	LevelDesc       []string
}

func NewPlainFormatter() *PlainFormatter {
	plainFormatter := new(PlainFormatter)
	plainFormatter.TimestampFormat = "2006-01-02 15:04:05"
	plainFormatter.LevelDesc = []string{"PANIC", "FATL ", "ERROR", "WARN ", "INFO ", "DEBUG"}
	return plainFormatter
}

func (f *PlainFormatter) Format(entry *log.Entry) ([]byte, error) {
	timestamp := fmt.Sprintf(entry.Time.Format(f.TimestampFormat))
	return []byte(fmt.Sprintf("%s %s %s\n", f.LevelDesc[entry.Level], timestamp, entry.Message)), nil
}
