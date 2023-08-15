package main

import (
	"os"
	"strings"

	"github.com/ListenOcean/goHookTool/internal/build"
	"github.com/ListenOcean/goHookTool/internal/build/log"
	"github.com/ListenOcean/goHookTool/internal/toolexec"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:               "autobuild",
	Short:             "Build a go application with hook.",
	TraverseChildren:  true,
	DisableAutoGenTag: true,
}

func init() {
	rootCmd.AddCommand(toolexec.ToolexecCmd)
	rootCmd.AddCommand(build.BuildCmd)
}

var NeedLog bool
var NeedUpdate bool

func main() {
	if len(os.Args) >= 2 {
		if os.Args[1] == "build" || os.Args[1] == "install" {
			NeedLog = true
		}
	}

	// 初始化日志
	if NeedLog {
		log.InitLog()
		log.Debug("Program Args.", log.String("args", strings.Join(os.Args, ", ")))
	}

	_ = rootCmd.Execute()
	// 同步日志，有检查可以直接调
	log.Sync()
	log.Clear()
}
