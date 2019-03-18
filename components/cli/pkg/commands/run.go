/*
 * Copyright (c) 2018 WSO2 Inc. (http:www.wso2.org) All Rights Reserved.
 *
 * WSO2 Inc. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http:www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package commands

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cellery-io/sdk/components/cli/pkg/constants"
	"github.com/cellery-io/sdk/components/cli/pkg/util"
)

func RunRun(cellImageTag string, instanceName string, dependencies []string) {
	parsedCellImage, err := util.ParseImageTag(cellImageTag)
	if err != nil {
		util.ExitWithErrorMessage("Error occurred while parsing cell image", err)
	}

	repoLocation := filepath.Join(util.UserHomeDir(), constants.CELLERY_HOME, "repo", parsedCellImage.Organization,
		parsedCellImage.ImageName, parsedCellImage.ImageVersion)
	fmt.Printf("Running cell image: %s\n", util.Bold(cellImageTag))
	zipLocation := filepath.Join(repoLocation, parsedCellImage.ImageName+constants.CELL_IMAGE_EXT)

	if _, err := os.Stat(zipLocation); os.IsNotExist(err) {
		fmt.Printf("\nUnable to find image %s locally.", cellImageTag)
		fmt.Printf("\nPulling image: %s", cellImageTag)
		RunPull(cellImageTag, true)
	}

	// Create tmp directory
	tmpPath := filepath.Join(util.UserHomeDir(), constants.CELLERY_HOME, "tmp", parsedCellImage.ImageName)
	err = util.CleanOrCreateDir(tmpPath)
	if err != nil {
		panic(err)
	}

	err = util.Unzip(zipLocation, tmpPath)
	if err != nil {
		panic(err)
	}

	if err != nil {
		util.ExitWithErrorMessage("Error occurred while extracting cell image", err)
	}
	balFileName, err := util.GetSourceFileName(filepath.Join(tmpPath, constants.ZIP_BALLERINA_SOURCE))
	if err != nil {
		util.ExitWithErrorMessage("Error occurred while extracting source file: ", err)
	}
	balFilePath := filepath.Join(tmpPath, constants.ZIP_BALLERINA_SOURCE, balFileName)
	containsRunFunction, err := util.RunMethodExists(balFilePath)
	if err != nil {
		util.ExitWithErrorMessage("Error occurred while checking for run function ", err)
	}
	if containsRunFunction {
		// Ballerina run method should be executed.
		if instanceName == "" {
			//Instance name not provided setting default {cellImageName}
			instanceName = parsedCellImage.ImageName
		}
		cmd := exec.Command("ballerina", "run", balFilePath+":run",
			parsedCellImage.Organization+"/"+parsedCellImage.ImageName,
			parsedCellImage.ImageVersion,
			instanceName, generateBalCompatibleMap(dependencies))
		stdoutReader, _ := cmd.StdoutPipe()
		stdoutScanner := bufio.NewScanner(stdoutReader)
		go func() {
			for stdoutScanner.Scan() {
				fmt.Printf("\033[36m%s\033[m\n", stdoutScanner.Text())
			}
		}()
		stderrReader, _ := cmd.StderrPipe()
		stderrScanner := bufio.NewScanner(stderrReader)
		go func() {
			for stderrScanner.Scan() {
				fmt.Printf("\033[36m%s\033[m\n", stderrScanner.Text())
			}
		}()
		err = cmd.Start()
		if err != nil {
			util.ExitWithErrorMessage("Error in executing cellery run", err)
		}
		err = cmd.Wait()
		if err != nil {
			util.ExitWithErrorMessage("Error occurred in cellery run", err)
		}
	}

	// Update the instance name
	kubeYamlDir := filepath.Join(tmpPath, constants.ZIP_ARTIFACTS, "cellery")
	kubeYamlFile := filepath.Join(kubeYamlDir, parsedCellImage.ImageName+".yaml")
	if instanceName != "" {
		//Cell instance name changed.
		err = util.ReplaceInFile(kubeYamlFile, "name: "+parsedCellImage.ImageName, "name: "+instanceName, 1)
	}
	if err != nil {
		util.ExitWithErrorMessage("Error in replacing cell instance name", err)
	}

	cmd := exec.Command("kubectl", "apply", "-f", kubeYamlDir)
	stdoutReader, _ := cmd.StdoutPipe()
	stdoutScanner := bufio.NewScanner(stdoutReader)
	go func() {
		for stdoutScanner.Scan() {
			fmt.Printf("\033[36m%s\033[m\n", stdoutScanner.Text())
		}
	}()
	stderrReader, _ := cmd.StderrPipe()
	stderrScanner := bufio.NewScanner(stderrReader)
	go func() {
		for stderrScanner.Scan() {
			fmt.Printf("\033[36m%s\033[m\n", stderrScanner.Text())
		}
	}()
	err = cmd.Start()
	if err != nil {
		util.ExitWithErrorMessage("Error in executing cell run", err)
	}
	err = cmd.Wait()
	_ = os.RemoveAll(kubeYamlDir)
	_ = os.RemoveAll(tmpPath)

	if err != nil {
		util.ExitWithErrorMessage("Error occurred while running cell image", err)
	}

	util.PrintSuccessMessage(fmt.Sprintf("Successfully deployed cell image: %s", util.Bold(cellImageTag)))
	util.PrintWhatsNextMessage("list running cells", "cellery list instances")
}

func generateBalCompatibleMap(depArr []string) string {
	var strBuffer bytes.Buffer
	strBuffer.WriteString("\"{")
	for index, element := range depArr {
		if index > 0 {
			strBuffer.WriteString(",")
		}
		depElements := strings.Split(element, ":")
		strBuffer.WriteString("\"")
		strBuffer.WriteString(depElements[0])
		strBuffer.WriteString("\"")
		strBuffer.WriteString(":")
		strBuffer.WriteString("\"")
		strBuffer.WriteString(depElements[1])
		strBuffer.WriteString("\"")
	}
	strBuffer.WriteString("}\"")
	return strBuffer.String()
}
