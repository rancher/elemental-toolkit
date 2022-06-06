package main

import (
	"fmt"
	"github.com/mudler/luet/pkg/api/client"
	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/mudler/luet/pkg/installer"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

// getRepo just gets the repo with the proper reference ID, syncs it and returns the repo pointer
func getRepo(repo string, ctx *types.Context) *installer.LuetSystemRepository {
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
	// Copy the reference as it's lost after sync :o
	re.ReferenceID = d.ReferenceID
	if err != nil {
		panic(err)
	}
	return re
}

// getRepositoryPackages gets all the packages in the repo and returns a SearchResult
func getRepositoryPackages(repo *installer.LuetSystemRepository) (searchResult client.SearchResult) {
	for _, p := range repo.GetTree().GetDatabase().World() {
		searchResult.Packages = append(searchResult.Packages, client.Package{
			Name:     p.GetName(),
			Category: p.GetCategory(),
			Version:  p.GetVersion(),
		})
	}
	return
}

// getRepositoryFiles returns the files that are part of the repo skeleton, not packages per se
func getRepositoryFiles(repo *installer.LuetSystemRepository) (repoFiles []string) {
	for _, f := range repo.RepositoryFiles {
		repoFiles = append(repoFiles, f.FileName)
	}
	// Don't forget to add the own repository.yaml file or equivalent!
	repoFiles = append(repoFiles, repo.ReferenceID)
	return
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
	repo := getRepo(finalRepo, ctx)
	packages := getRepositoryPackages(repo)
	for _, val := range packages.Packages {
		imageTag := fmt.Sprintf("%s:%s", finalRepo, val.ImageTag())
		checkAndSign(imageTag, ctx)
	}
	repoFiles := getRepositoryFiles(repo)
	for _, val := range repoFiles {
		imageTag := fmt.Sprintf("%s:%s", finalRepo, val)
		checkAndSign(imageTag, ctx)
	}
	return
}

func checkAndSign(tag string, ctx *types.Context) {
	var fulcioFlag string
	var args []string
	tag = strings.TrimSpace(tag)

	ctx.Info("Checking artifact", tag)
	tmpDir, _ := os.MkdirTemp("", "sign-*")
	defer os.RemoveAll(tmpDir)

	_ = os.Setenv("TUF_ROOT", tmpDir)         // TUF_DIR per run, we dont want to access the same files as another process
	_ = os.Setenv("COSIGN_EXPERIMENTAL", "1") // Set keyless verify/sign

	fulcioURL := os.Getenv("FULCIO_URL") // Allow to set a fulcio url
	if fulcioURL != "" {
		fulcioFlag = fmt.Sprintf("--fulcio-url=%s", strings.TrimSpace(fulcioURL))
		ctx.Info("Found FULCIO_URL var, setting the following flag to the cosing command:", fulcioFlag)
	}
	args = []string{"verify", tag}
	ctx.Debug("Calling cosing with the following args:", args)
	out, err := exec.Command("cosign", args...).CombinedOutput()
	ctx.Debug("Verify output:", string(out))
	if err != nil {
		ctx.Warning("Artifact", tag, "has no signature, signing it")
		if fulcioFlag != "" {
			args = []string{"sign", fulcioFlag, tag}
		} else {
			args = []string{"sign", tag}
		}
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
