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
	"bufio"
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
)

type EnumType interface {
	values() interface{}
}

// Find takes a slice and looks for an element in it. If found it will
// return the item founc, otherwise it will return nil.
func Find(slice []interface{}, val interface{}) interface{} {
	for item := range slice {
		if item == val {
			return item
		}
	}
	return nil
}

// Find takes a slice and looks for an element using the given predicate in it.
// If found it will return the item found, otherwise nil.
func FindWithPredicate(slice []interface{}, predicate func(interface{}) bool) interface{} {
	s := reflect.ValueOf(slice)

	if s.Kind() != reflect.Slice {
		panic("Invalid data-type")
	}
	for item := range slice {
		if predicate(item) {
			return item
		}
	}
	return nil
}

func IndexOf(slice interface{}, item interface{}) int {
	s := reflect.ValueOf(slice)

	if s.Kind() != reflect.Slice {
		panic("Invalid data-type")
	}

	for i := 0; i < s.Len(); i++ {
		if s.Index(i).Interface() == item {
			return i
		}
	}
	return -1
}

func Exists(slice interface{}, item interface{}) bool {
	return IndexOf(slice, item) != -1
}

func Keys(mymap map[interface{}]interface{}) []interface{} {
	keys := []interface{}{}
	for k := range mymap {
		keys = append(keys, k)
	}
	return keys
}

// Exists reports whether the named file or directory exists.
func FileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

/**
 * Runs an OS command.
 */
func RunCommand(description string, command string, args string) (*exec.Cmd, io.ReadCloser, error) {
	fmt.Printf(description)
	cmd := exec.Command(command, args)
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error creating StdoutPipe for Cmd: "+command, err)
	}
	err = cmd.Start()
	return cmd, cmdReader, err
}

func FollowCommandWait(cmdReader io.ReadCloser, print bool) []byte {
	scanner := bufio.NewScanner(cmdReader)
	var b bytes.Buffer
	for scanner.Scan() {
		b.WriteString(scanner.Text())
	}
	return b.Bytes()
}

func FollowCommandAsynch(cmdReader io.ReadCloser, buffer bytes.Buffer, print bool) {
	scanner := bufio.NewScanner(cmdReader)
	go func() {
		for scanner.Scan() {
			text := scanner.Text()
			buffer.WriteString(text)
			if print {
				fmt.Printf("\t > %s\n", text)
			}
		}
	}()
}

func PanicIfError(err error) {
	if err != nil {
		panic(err)
	}
}

// Checks if an error occurred and logs any with the given area
// using the current logger on ERROR level.
// Returns true, if no error is present
func CheckAndLogError(area string, err error) bool {
	if err != nil {
		log.Error(area + ": " + err.Error())
		return false
	}
	return true
}

// ParseBool returns the boolean value represented by the string.
// It accepts 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False.
// Any other value returns an error.
func ParseBool(str string) bool {
	switch str {
	case "1", "t", "T", "true", "TRUE", "True", "on", "On", "ON":
		return true
	case "0", "f", "F", "false", "FALSE", "False", "off", "Off", "OFF":
		return false
	default:
		return false
	}
}

// Calculates a runtme info with OS/Architecture
func RuntimeInfo() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

type Properties map[string]string

func ReadProperties(filename string) (Properties, error) {
	config := Properties{}

	if len(filename) == 0 {
		return config, nil
	}
	file, err := os.Open(filename)
	if err != nil {
		log.Info(err)
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if equal := strings.Index(line, "="); equal >= 0 {
			if key := strings.TrimSpace(line[:equal]); len(key) > 0 {
				value := ""
				if len(line) > equal {
					value = strings.TrimSpace(line[equal+1:])
				}
				config[key] = value
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
		return nil, err
	}

	return config, nil
}

func ParseURL(urlString string) *url.URL {
	urlParsed, err := url.ParseRequestURI(urlString)
	if err != nil {
		fmt.Println("Failed to parse URL from  %v, error: %v", urlString, err)
	}
	return urlParsed
}
