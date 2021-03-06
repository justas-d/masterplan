package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"github.com/otiai10/copy"
)

func build(baseDir string, ldFlags string) bool {

	fmt.Println(fmt.Sprintf("Beginning build to %s.", baseDir))

	copyTo := func(src, dest string) {
		if err := copy.Copy(src, dest); err != nil {
			panic(err)
		}
	}

	copyTo("assets", filepath.Join(baseDir, "assets"))

	fmt.Println("Assets copied.")

	filename := filepath.Join(baseDir, "MasterPlan")

	args := []string{"build", "-ldflags", ldFlags, "-o", filename, "./"}

	fmt.Println(fmt.Sprintf("Building binary with flags %s...", args))

	result, err := exec.Command("go", args...).CombinedOutput()

	if string(result) != "" {
		fmt.Println(string(result))
	}

	os.Chmod(filename, 0777)

	if err == nil {
		fmt.Println("Build complete!")
		fmt.Println("")

    return true
	}

  return false
}

func main() {
	if build("bin", "-X main.releaseMode=true") {
    // NOTE(justasd): console file handles aren't properly redirected to the parent terminal if we do
    // this.
    //exec.Command("bin/MasterPlan", "").CombinedOutput()
  }
}


