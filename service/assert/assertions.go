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

package assert

//Provides several simple assertions that are handy in a daily progrtammers life.

// Asserts the value is not nil, panics if so.
func AssertNotNil(instance interface{}) interface{} {
	return AssertNotNilWithMsg(instance, "Instance is nil")
}

// Asserts the value is not nil, panics if so with the given message.
func AssertNotNilWithMsg(instance interface{}, message string) interface{} {
	if instance == nil {
		if message == "" {
			panic("Instance is nil")
		} else {
			panic(message)
		}
	}
	return instance
}
