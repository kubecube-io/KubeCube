/*
Copyright 2021 KubeCube Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/kubecube-io/kubecube/cmd/cube/app/options/flags"

	cube "github.com/kubecube-io/kubecube/cmd/cube/app"

	"github.com/urfave/cli/v2"
)

var version = "1.0.0-rc0"

func main() {
	app := cli.NewApp()
	app.Name = "KubeCube"
	app.Usage = "KubCube is foundation of the world upon"
	app.Version = version
	app.Compiled = time.Now()
	app.Copyright = "(c) " + strconv.Itoa(time.Now().Year()) + " KubeCube"

	app.Flags = flags.Flags
	app.Before = cube.Before
	app.Action = cube.Start

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
