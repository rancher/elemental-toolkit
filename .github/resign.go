package main

import (
	"fmt"
	"github.com/mudler/luet/pkg/api/client"
	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/mudler/luet/pkg/installer"
	"io/ioutil"
	"os"
	"os/exec"
)

func getRepositoryPackages(repo string, ctx *types.Context) (searchResult client.SearchResult) {
	tmpdir, err := ioutil.TempDir(os.TempDir(), "ci")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpdir)
	referenceID := os.Getenv("REFERENCEID")
	if referenceID == "" {
		referenceID = "repository.yaml"
	}
	d := installer.NewSystemRepository(types.LuetRepository{
		Name:        "cOS",
		Type:        "docker",
		Cached:      true,
		Urls:        []string{repo},
		ReferenceID: referenceID,
	})
	ctx.Config.GetSystem().Rootfs = "/"
	ctx.Config.GetSystem().TmpDirBase = tmpdir
	re, err := d.Sync(ctx, false)
	if err != nil {
		panic(err)
	} else {
		for _, p := range re.GetTree().GetDatabase().World() {
			searchResult.Packages = append(searchResult.Packages, client.Package{
				Name:     p.GetName(),
				Category: p.GetCategory(),
				Version:  p.GetVersion(),
			})
		}
		return
	}
}

func main() {
	// We want to be cool and keep the same format as luet, so we create the context here to pass around and use the logging functions
	ctx := types.NewContext()
	if d := os.Getenv("DEBUGLOGLEVEL"); d != "" && d != "false" {
		ctx.Config.GetGeneral().Debug = true
	}
	finalRepo := os.Getenv("FINAL_REPO")
	if finalRepo == "" {
		ctx.Error("A container repository must be specified with FINAL_REPO")
		os.Exit(1)
	}
	cosignRepo := os.Getenv("COSIGN_REPOSITORY")
	if cosignRepo == "" {
		ctx.Error("A signature repository must be specified with COSIGN_REPOSITORY")
		os.Exit(1)
	}
	packages := getRepositoryPackages(finalRepo, ctx)
	for _, val := range packages.Packages {
		imageTag := fmt.Sprintf("%s:%s", finalRepo, val.ImageTag())
		checkAndSign(imageTag, ctx)
	}
	return
}

func checkAndSign(tag string, ctx *types.Context) {
	var fulcioFlag string

	ctx.Info("Checking artifact", tag)
	tmpDir, _ := os.MkdirTemp("", "sign-*")
	defer os.RemoveAll(tmpDir)

	_ = os.Setenv("TUF_ROOT", tmpDir)         // TUF_DIR per run, we dont want to access the same files as another process
	_ = os.Setenv("COSIGN_EXPERIMENTAL", "1") // Set keyless verify/sign

	fulcioURL := os.Getenv("FULCIO_URL") // Allow to set a fulcio url
	if fulcioURL != "" {
		fulcioFlag = fmt.Sprintf("--fulcio-url=%s", fulcioURL)
		ctx.Info("Found FULCIO_URL var, setting the following flag to the cosing command:", fulcioFlag)
	}

	args := []string{"verify", tag}
	ctx.Debug("Calling cosing with the following args:", args)
	out, err := exec.Command("cosign", args...).CombinedOutput()
	ctx.Debug("Verify output:", string(out))
	if err != nil {
		ctx.Warning("Artifact", tag, "has no signature, signing it")
		args := []string{fulcioFlag, "sign", tag}
		ctx.Debug("Calling cosing with the following args:", args)
		out, err := exec.Command("cosign", args...).CombinedOutput()
		if err != nil {
			ctx.Error("Error signing", tag, ":", err)
			ctx.Debug("Error signing", tag, "output:", string(out))
		} else {
			ctx.Success("Artifact", tag, "signed")
		}
	} else {
		ctx.Success("Artifact", tag, "has signature")
	}
}
