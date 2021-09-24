package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
)

const DefaultRetries = 120

type opData struct {
	FinalRepo string
}

type resultData struct {
	Package Package
	Exists  bool
}

func retryDownload(img, dest string, t int) error {
	if err := download(img, dest); err != nil {
		if t <= 0 {
			return err
		}
		fmt.Printf("failed downloading '%s', retrying..\n", img)
		time.Sleep(time.Duration(DefaultRetries-t+1) * time.Second)
		return retryDownload(img, dest, t-1)
	}
	return nil
}

func retryList(image string, t int) ([]string, error) {
	tags, err := crane.ListTags(image)
	if err != nil {
		if t <= 0 {
			return tags, err
		}
		fmt.Printf("failed listing tags for '%s', retrying..\n", image)
		time.Sleep(time.Duration(DefaultRetries-t+1) * time.Second)
		return retryList(image, t-1)
	}

	return tags, nil
}

func imageTags(tag string) ([]string, error) {
	return retryList(tag, DefaultRetries)
}

func download(img, dst string) error {
	tmpdir, err := ioutil.TempDir(os.TempDir(), "ci")
	if err != nil {
		return err
	}
	unpackdir, err := ioutil.TempDir(os.TempDir(), "ci")
	if err != nil {
		return err
	}
	err = RunSH("unpack", fmt.Sprintf("TMPDIR=%s XDG_RUNTIME_DIR=%s luet util unpack %s %s", tmpdir, tmpdir, img, unpackdir))
	if err != nil {
		return err
	}
	err = RunSH("move", fmt.Sprintf("mv %s/* %s/", unpackdir, dst))
	if err != nil {
		return err
	}
	os.RemoveAll(tmpdir)
	os.RemoveAll(unpackdir)
	return nil
}

func downloadImage(img, dst string) error {
	return retryDownload(img, dst, DefaultRetries)
}

func downloadMeta(p Package, o opData) error {
	return downloadImage(p.ImageMetadata(o.FinalRepo), "build")
}

func main() {
	finalRepo := os.Getenv("FINAL_REPO")
	if finalRepo == "" {
		fmt.Println("A container repository must be specified with FINAL_REPO")
		os.Exit(1)
	}

	buildScript := os.Getenv("BUILD_SCRIPT")
	if buildScript == "" {
		buildScript = "./.github/build.sh"
	}
	packs, err := TreePackages("./packages")
	checkErr(err)
	missingPackages := []Package{}

	tags, err := imageTags(finalRepo)
	checkErr(err)

	for _, p := range packs.Packages {
		if !containsS(p.ImageTag(), tags) {
			missingPackages = append(missingPackages, p)
		}
	}

	fmt.Println("Missing packages: " + fmt.Sprint(len(missingPackages)))
	for _, m := range missingPackages {
		fmt.Println("-", m.String())
	}

	if os.Getenv("SKIP_PACKAGES") != "" {
		for _, skip := range strings.Split(os.Getenv("SKIP_PACKAGES"), " ") {
			for index, pkg := range missingPackages {
				name := fmt.Sprintf("%s/%s", pkg.Category, pkg.Name)
				if name == skip {
					fmt.Println("- Skipping build of package due to SKIP_PACKAGES: ", pkg.String())
					// how absurd is this just to pop one element from a slice ¬_¬
					missingPackages[index] = missingPackages[len(missingPackages)-1] // Copy last element to index i.
					missingPackages[len(missingPackages)-1] = Package{}              // Erase last element (write empty value).
					missingPackages = missingPackages[:len(missingPackages)-1]       // Truncate slice.
				}
			}
		}
	}

	if os.Getenv("DOWNLOAD_ONLY") != "true" {
		for _, m := range missingPackages {
			fmt.Println("Building", m.String())
			checkErr(RunSH("build", fmt.Sprintf("%s %s", buildScript, m.String())))
		}
	}

	if os.Getenv("DOWNLOAD_METADATA") == "true" {
		fmt.Println("Populating build folder with metadata files")
		op := opData{FinalRepo: finalRepo}

		if os.Getenv("DOWNLOAD_ALL") == "true" {
			fmt.Println("Downloading all available metadata files on the remote repository")

			for _, t := range tags {
				if strings.HasSuffix(t, ".metadata.yaml") {
					img := fmt.Sprintf("%s:%s", finalRepo, t)
					fmt.Println("Downloading", img)
					checkErr(
						downloadImage(img, "build"),
					)

				}
			}
		} else {
			for _, p := range packs.Packages {
				if !contains(missingPackages, p) {
					err := downloadMeta(p, op)
					checkErr(err)
				}
			}
		}

	}
}

func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func RunSHOUT(stepName, bashFragment string) ([]byte, error) {
	cmd := exec.Command("sh", "-s")
	cmd.Stdin = strings.NewReader(bashWrap(bashFragment))

	cmd.Env = os.Environ()
	//	log.Printf("Running in background: %v", stepName)

	return cmd.CombinedOutput()
}

func RunSH(stepName, bashFragment string) error {
	cmd := exec.Command("sh", "-s")
	cmd.Stdin = strings.NewReader(bashWrap(bashFragment))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	log.Printf("Running: %v (%v)", stepName, bashFragment)

	return cmd.Run()
}

func bashWrap(cmd string) string {
	return `
set -o errexit
set -o nounset
` + cmd + `
`
}

type SearchResult struct {
	Packages []Package
}

type Package struct {
	Name, Category, Version, Path string
}

func TreePackages(treedir string) (searchResult SearchResult, err error) {
	var res []byte
	res, err = RunSHOUT("tree", fmt.Sprintf("luet tree pkglist --tree %s --output json", treedir))
	if err != nil {
		fmt.Println(string(res))
		return
	}
	json.Unmarshal(res, &searchResult)
	return
}

func imageAvailable(image string) bool {
	_, err := crane.Digest(image)
	return err == nil
}

func contains(pp []Package, p Package) bool {
	for _, i := range pp {
		if i.Equal(p) {
			return true
		}
	}
	return false
}

func containsS(s string, slice []string) bool {
	for _, i := range slice {
		if s == i {
			return true
		}
	}
	return false
}

func (p Package) String() string {
	return fmt.Sprintf("%s/%s@%s", p.Category, p.Name, p.Version)
}

func (p Package) Image(repository string) string {
	// ${name}-${category}-${version//+/-}
	return fmt.Sprintf("%s:%s", repository, p.ImageTag())
}

func (p Package) ImageTag() string {
	// ${name}-${category}-${version//+/-}
	return fmt.Sprintf("%s-%s-%s", p.Name, p.Category, strings.ReplaceAll(p.Version, "+", "-"))
}

func (p Package) ImageMetadata(repository string) string {
	// ${name}-${category}-${version//+/-}
	return fmt.Sprintf("%s.metadata.yaml", p.Image(repository))
}

func (p Package) ImageAvailable(repository string) bool {
	return imageAvailable(p.Image(repository))
}

func (p Package) Equal(pp Package) bool {
	return p.Name == pp.Name && p.Category == pp.Category && p.Version == pp.Version
}

func (p Package) EqualS(s string) bool {
	return s == fmt.Sprintf("%s/%s", p.Category, p.Name)
}

func (p Package) EqualNoV(pp Package) bool {
	return p.Name == pp.Name && p.Category == pp.Category
}

func (s SearchResult) FilterByCategory(cat string) SearchResult {
	new := SearchResult{Packages: []Package{}}

	for _, r := range s.Packages {
		if r.Category == cat {
			new.Packages = append(new.Packages, r)
		}
	}
	return new
}

func (s SearchResult) FilterByName(name string) SearchResult {
	new := SearchResult{Packages: []Package{}}

	for _, r := range s.Packages {
		if !strings.Contains(r.Name, name) {
			new.Packages = append(new.Packages, r)
		}
	}
	return new
}
