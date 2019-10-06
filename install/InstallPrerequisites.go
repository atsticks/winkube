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

/**
 * Installation command, which checks fir the prerequisites ad^nd installs everything missing. This command should be
 * executed with administrative right.
 */
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

// command to install chocolatly with Windows powershell
const chocolatlyInstallArg string = "Set-ExecutionPolicy Bypass -Scope Process -Force; iex ((New-Object System.Net.WebClient).DownloadString('https://chocolatey.org/install.ps1'))"

func main() {
	fmt.Println("*********************************************\n" +
		"**  Windows Kubernetes Installer, v 0.0.1\n" +
		"**********************************************\n\n" +
		"Checking prerequisites...")
	fmt.Println()
	if !check("choco") {
		install("Chocolatey package manager", "powershell.exe", chocolatlyInstallArg)
	} else {
		fmt.Println("Chocolatey package manager found.")
	}
	if !check("C:\\Program Files\\Oracle\\VirtualBox\\VirtualBox.exe") {
		install("Virtualbox", "choco", "install -y virtualbox")
	} else {
		fmt.Println("Virtualbox found.")
	}
	if !check("vagrant") {
		install("Vagrant", "choco", "install -y vagrant")
	} else {
		fmt.Println("Vagrant found.")
	}
	fmt.Println("All prerequisites are ready, starting visual installer...")
}

/**
 * Installs the declared package tith the given command string and arguments.
 */
func install(declaredPackage string, command string, args string) {
	fmt.Printf("Installing %v ...\n", declaredPackage)
	cmd := exec.Command(command, args)
	err := cmd.Run()
	if err != nil {
		log.Panic("Installation of "+declaredPackage+" failed, aborting...", err)
	} else {
		fmt.Printf("%v installed successfully.\n", declaredPackage)
	}
}

func check(command string) bool {
	fmt.Printf("Checking if %v is available...\n", command)
	if fileExists(command) {
		return true
	}
	cmd := exec.Command("where", command)
	err := cmd.Run()

	return err == nil
}

// Exists reports whether the named file or directory exists.
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
