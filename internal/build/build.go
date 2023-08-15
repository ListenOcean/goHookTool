package build

import (
	"os"
	"os/exec"

	"github.com/ListenOcean/goHookTool/configs"
	"github.com/ListenOcean/goHookTool/internal/build/log"

	"github.com/spf13/cobra"
)

var (
	WorkDir       string
	GoPath        string
	AutobuildPath string
)

// buildCmd represents the build command
var BuildCmd = &cobra.Command{
	Use:                "build",
	Short:              "Build a go application with hook.",
	RunE:               BuildEntry,
	DisableFlagParsing: true,
}

func BuildEntry(cmd *cobra.Command, args []string) (err error) {
	if WorkDir, err = os.Getwd(); err != nil {
		return
	}

	if GoPath, err = exec.LookPath("go"); err != nil {
		return
	}

	if AutobuildPath, err = os.Executable(); err != nil {
		return
	}

	customGoBin := os.Getenv(configs.TagCustomGoBin)
	if customGoBin != "" {
		GoPath = customGoBin
	}

	if err = ForwardBuild(); err != nil {
		log.Error("ForwardBuild Fail.", log.String("err", err.Error()))
		return
	}
	return nil
}
