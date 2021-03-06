// Copyright 2015 - 2017 Ka-Hing Cheung
// Copyright 2015 - 2017 Google Inc. All Rights Reserved.
// Modifications Copyright 2018 The MITRE Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mattrbianchi/twig"
	"github.com/mitre/fusera/nr"
	"github.com/pkg/errors"

	"github.com/urfave/cli"
)

var Version = "beta"
var flags *Flags

func init() {
	twig.SetFlags(twig.LstdFlags | twig.Lshortfile)
}

func main() {
	VersionHash = Version
	EnsurePathIsSet()
	var app = NewApp()
	app.Action = func(c *cli.Context) error {
		if c.IsSet("help") {
			cli.ShowAppHelpAndExit(c, 0)
		}
		// Populate and parse flags.
		flags, err := PopulateFlags(c)
		if err != nil {
			cause := errors.Cause(err)
			if os.IsPermission(cause) {
				fmt.Printf("\nSeems like %s doesn't have permissions to read a file!\n", app.Name)
				fmt.Println("Try changing the permissions with chmod +r path/to/file")
				fmt.Println("")
			}
			fmt.Printf("\ninvalid arguments: %s\n\n", errors.Cause(err))
			twig.Debugf("%+#v", err.Error())
			cli.ShowAppHelpAndExit(c, 1)
		}
		twig.Debugf("accs: %s", flags.Acc)
		// TODO: go ask for URLs, run libcurl
		accs, err := nr.ResolveNames(flags.Endpoint, flags.Loc, flags.Ngc, flags.Acc)
		if err != nil {
			return err
		}
		_, err = exec.LookPath("curl")
		if err != nil {
			// TODO: create better message describing that curl isnt installed
			return err
		}
		for _, v := range accs {
			err := os.Mkdir(filepath.Join(flags.Path, v.ID), 0755)
			if err != nil {
				twig.Infof("Issue creating directory for %s: %s\n", v.ID, err.Error())
				continue
			}
			for _, f := range v.Files {
				if c.IsSet("only") {
					ext := filepath.Ext(f.Name)
					ext = strings.TrimLeft(ext, ".")
					if _, ok := flags.Types[ext]; !ok {
						continue
					}
				}
				// TODO: call libcurl on each url to the path specified
				args := []string{"-o", filepath.Join(flags.Path, v.ID, f.Name), f.Link}
				cmd := exec.Command("curl", args...)
				cmd.Env = os.Environ()
				err := cmd.Run()
				if err != nil {
					twig.Infof("Issue copying %s: %s\n", args[2], err.Error())
				}
			}
		}
		return nil
	}
	err := app.Run(os.Args)
	if err != nil {
		twig.Infof("Error running command: %s\n", err.Error())
		os.Exit(1)
	}
}

// mount -a seems to run goofys without PATH
// usually fusermount is in /bin
func EnsurePathIsSet() {
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "PATH=") {
			return
		}
	}

	os.Setenv("PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
}
