// Copyright 2020 Clivern. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/clivern/penguin/core/controller"
	"github.com/clivern/penguin/core/middleware"
	"github.com/clivern/penguin/core/util"

	"github.com/drone/envsubst"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var config string

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run penguin server",
	Run: func(cmd *cobra.Command, args []string) {
		var runerr error

		configUnparsed, err := ioutil.ReadFile(config)

		if err != nil {
			panic(fmt.Sprintf(
				"Error while reading config file [%s]: %s",
				config,
				err.Error(),
			))
		}

		configParsed, err := envsubst.EvalEnv(string(configUnparsed))

		if err != nil {
			panic(fmt.Sprintf(
				"Error while parsing config file [%s]: %s",
				config,
				err.Error(),
			))
		}

		viper.SetConfigType("yaml")
		err = viper.ReadConfig(bytes.NewBuffer([]byte(configParsed)))

		if err != nil {
			panic(fmt.Sprintf(
				"Error while loading configs [%s]: %s",
				config,
				err.Error(),
			))
		}

		if viper.GetString("log.output") != "stdout" {
			dir, _ := filepath.Split(viper.GetString("log.output"))

			if !util.DirExists(dir) {
				if _, err := util.EnsureDir(dir, 777); err != nil {
					panic(fmt.Sprintf(
						"Directory [%s] creation failed with error: %s",
						dir,
						err.Error(),
					))
				}
			}

			if !util.FileExists(viper.GetString("log.output")) {
				f, err := os.Create(viper.GetString("log.output"))
				if err != nil {
					panic(fmt.Sprintf(
						"Error while creating log file [%s]: %s",
						viper.GetString("log.output"),
						err.Error(),
					))
				}
				defer f.Close()
			}
		}

		if viper.GetString("log.output") == "stdout" {
			gin.DefaultWriter = os.Stdout
			log.SetOutput(os.Stdout)
		} else {
			f, _ := os.Create(viper.GetString("log.output"))
			gin.DefaultWriter = io.MultiWriter(f)
			log.SetOutput(f)
		}

		lvl := strings.ToLower(viper.GetString("log.level"))
		level, err := log.ParseLevel(lvl)

		if err != nil {
			level = log.InfoLevel
		}

		log.SetLevel(level)

		if viper.GetString("app.mode") == "prod" {
			gin.SetMode(gin.ReleaseMode)
			gin.DefaultWriter = ioutil.Discard
			gin.DisableConsoleColor()
		}

		if viper.GetString("log.format") == "json" {
			log.SetFormatter(&log.JSONFormatter{})
		} else {
			log.SetFormatter(&log.TextFormatter{})
		}

		messages := make(chan string, 5000)

		go controller.Daemon(messages)

		// If http input not enabled & log watcher enabled
		if !viper.GetBool("inputs.http.enabled") && viper.GetBool("inputs.log.enabled") {
			controller.Watcher(messages)
			return
		} else if viper.GetBool("inputs.log.enabled") {
			go controller.Watcher(messages)
		}

		r := gin.Default()

		r.Use(middleware.Correlation())
		r.Use(middleware.Auth())
		r.Use(middleware.Logger())
		r.Use(middleware.Metric())

		r.GET("/favicon.ico", func(c *gin.Context) {
			c.String(http.StatusNoContent, "")
		})

		r.GET("/_health", controller.HealthCheck)

		r.POST(viper.GetString("inputs.http.path"), func(c *gin.Context) {
			controller.Listener(c, messages)
		})

		r.GET(viper.GetString("output.prometheus.endpoint"), gin.WrapH(controller.Metrics()))

		if viper.GetBool("inputs.http.tls.status") {
			runerr = r.RunTLS(
				fmt.Sprintf(":%s", strconv.Itoa(viper.GetInt("inputs.http.port"))),
				viper.GetString("inputs.http.tls.pemPath"),
				viper.GetString("inputs.http.tls.keyPath"),
			)
		} else {
			runerr = r.Run(
				fmt.Sprintf(":%s", strconv.Itoa(viper.GetInt("inputs.http.port"))),
			)
		}

		if runerr != nil {
			panic(runerr.Error())
		}
	},
}

func init() {
	runCmd.Flags().StringVarP(
		&config,
		"config",
		"c",
		"config.prod.yml",
		"Absolute path to config file (required)",
	)
	runCmd.MarkFlagRequired("config")
	rootCmd.AddCommand(runCmd)
}
