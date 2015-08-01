// Command lifx allows basic performing operations on LIFX devices over the LAN
package main

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/pdf/golifx"
	"github.com/pdf/golifx/common"
	"github.com/pdf/golifx/protocol"
)

var (
	client *golifx.Client

	flagTimeout  time.Duration
	flagLogLevel string

	logger = logrus.New()
	app    = &cobra.Command{
		Use: `lifx`,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			setLogger()
		},
	}

	cmdGenerateBashComp = &cobra.Command{
		Use:   `bashcomp <filename>`,
		Short: "generate bash completion at <file>",
		Run:   generateBashComp,
	}

	cmdGenerateDocs = &cobra.Command{
		Use:   `docs <path>`,
		Short: "generate markdown documentation at <path>",
		Run:   generateDocs,
	}
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	golifx.SetLogger(logger)

	app.PersistentFlags().DurationVarP(&flagTimeout, `timeout`, `t`, common.DefaultTimeout, `timeout for all operations`)
	app.PersistentFlags().StringVarP(&flagLogLevel, `log-level`, `L`, `info`, `log level, one of: [debug,info,warn,error]`)

	app.AddCommand(cmdLight)
	app.AddCommand(cmdGenerateBashComp)
	app.AddCommand(cmdGenerateDocs)
}

func main() {
	app.Execute()
}

func setupClient(c *cobra.Command, args []string) {
	var err error

	client, err = golifx.NewClient(&protocol.V2{Reliable: true})
	if err != nil {
		logger.WithField(`error`, err).Fatalln(`Failed initializing client`)
	}
}

func closeClient(c *cobra.Command, args []string) {
	err := client.Close()
	if err != nil {
		logger.WithField(`error`, err).Fatalln(`Failed closing client`)
	}
}

func generateBashComp(c *cobra.Command, args []string) {
	if len(args) != 1 {
		c.Usage()
		fmt.Println()
		logger.Fatalln(`Missing filename`)
	}

	buf := new(bytes.Buffer)
	f, err := os.Create(args[0])
	if err != nil {
		logger.WithFields(logrus.Fields{
			`filename`: args[0],
			`error`:    err,
		}).Fatalln(`Could not open file`)
	}
	app.GenBashCompletion(buf)
	buf.WriteTo(f)
}

func generateDocs(c *cobra.Command, args []string) {
	if len(args) != 1 {
		c.Usage()
		fmt.Println()
		logger.Fatalln(`Missing output path`)
	}

	path := args[0]
	if path[len(path)-1] != os.PathSeparator {
		path += string(os.PathSeparator)
	}
	cobra.GenMarkdownTree(app, path)
}

func usage(c *cobra.Command, args []string) {
	c.Usage()
}

func setLogger() {
	switch flagLogLevel {
	case `debug`:
		logger.Level = logrus.DebugLevel
	case `info`:
		logger.Level = logrus.InfoLevel
	case `warn`:
		logger.Level = logrus.WarnLevel
	case `error`:
		logger.Level = logrus.ErrorLevel
	default:
		logger.Level = logrus.InfoLevel
	}
}
