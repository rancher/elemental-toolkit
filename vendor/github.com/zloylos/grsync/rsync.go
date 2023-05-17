package grsync

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Rsync is wrapper under rsync
type Rsync struct {
	Source      string
	Destination string

	cmd *exec.Cmd
}

// RsyncOptions for rsync
type RsyncOptions struct {
	// RsyncBinaryPath is a path to the rsync binary; by default just `rsync`
	RsyncBinaryPath string
	// RsyncPath specify the rsync to run on remote machine, e.g `--rsync-path="cd /a/b && rsync"`
	RsyncPath string
	// Verbose increase verbosity
	Verbose bool
	// Quet suppress non-error messages
	Quiet bool
	// Checksum skip based on checksum, not mod-time & size
	Checksum bool
	// Archve is archive mode; equals -rlptgoD (no -H,-A,-X)
	Archive bool
	// Recurse into directories
	Recursive bool
	// Relative option to use relative path names
	Relative bool
	// NoImliedDirs don't send implied dirs with --relative
	NoImpliedDirs bool
	// Update skip files that are newer on the receiver
	Update bool
	// Inplace update destination files in-place
	Inplace bool
	// Append data onto shorter files
	Append bool
	// AppendVerify --append w/old data in file checksum
	AppendVerify bool
	// Dirs transfer directories without recursing
	Dirs bool
	// Links copy symlinks as symlinks
	Links bool
	// CopyLinks transform symlink into referent file/dir
	CopyLinks bool
	// CopyUnsafeLinks only "unsafe" symlinks are transformed
	CopyUnsafeLinks bool
	// SafeLinks ignore symlinks that point outside the tree
	SafeLinks bool
	// CopyDirLinks transform symlink to dir into referent dir
	CopyDirLinks bool
	// KeepDirLinks treat symlinked dir on receiver as dir
	KeepDirLinks bool
	// HardLinks preserve hard links
	HardLinks bool
	// Perms preserve permissions
	Perms bool
	// NoPerms preserve permissions
	NoPerms bool
	// Executability preserve executability
	Executability bool
	// CHMOD affect file and/or directory permissions
	CHMOD os.FileMode
	// Acls preserve ACLs (implies -p)
	ACLs bool
	// XAttrs preserve extended attributes
	XAttrs bool
	// Owner preserve owner (super-user only)
	Owner bool
	// NoOwner prevent copying owner information to destination
	NoOwner bool
	// Group preserve group
	Group bool
	// NoGroup prevent copying group information to destination
	NoGroup bool
	// Devices preserve device files (super-user only)
	Devices bool
	// Specials preserve special files
	Specials bool
	// Times preserve modification times
	Times bool
	// NoTimes prevent copying modification times
	NoTimes bool
	// omit directories from --times
	OmitDirTimes bool
	// Super receiver attempts super-user activities
	Super bool
	// FakeSuper store/recover privileged attrs using xattrs
	FakeSuper bool
	// Sparce handle sparse files efficiently
	Sparse bool
	// DryRun perform a trial run with no changes made
	DryRun bool
	// WholeFile copy files whole (w/o delta-xfer algorithm)
	WholeFile bool
	// OneFileSystem don't cross filesystem boundaries
	OneFileSystem bool
	// BlockSize block-size=SIZE force a fixed checksum block-size
	BlockSize int
	// Rsh -rsh=COMMAND specify the remote shell to use
	Rsh string
	// Existing skip creating new files on receiver
	Existing bool
	// IgnoreExisting skip updating files that exist on receiver
	IgnoreExisting bool
	// RemoveSourceFiles sender removes synchronized files (non-dir)
	RemoveSourceFiles bool
	// Delete delete extraneous files from dest dirs
	Delete bool
	// DeleteBefore receiver deletes before transfer, not during
	DeleteBefore bool
	// DeleteDuring receiver deletes during the transfer
	DeleteDuring bool
	// DeleteDelay find deletions during, delete after
	DeleteDelay bool
	// DeleteAfter receiver deletes after transfer, not during
	DeleteAfter bool
	// DeleteExcluded also delete excluded files from dest dirs
	DeleteExcluded bool
	// IgnoreErrors delete even if there are I/O errors
	IgnoreErrors bool
	// Force deletion of dirs even if not empty
	Force bool
	// MaxDelete max-delete=NUM don't delete more than NUM files
	MaxDelete int
	// MaxSize max-size=SIZE don't transfer any file larger than SIZE
	MaxSize int
	// MinSize don't transfer any file smaller than SIZE
	MinSize int
	// Partial keep partially transferred files
	Partial bool
	// PartialDir partial-dir=DIR
	PartialDir string
	// DelayUpdates put all updated files into place at end
	DelayUpdates bool
	// PruneEmptyDirs prune empty directory chains from file-list
	PruneEmptyDirs bool
	// NumericIDs don't map uid/gid values by user/group name
	NumericIDs bool
	// Timeout timeout=SECONDS set I/O timeout in seconds
	Timeout int
	// Contimeout contimeout=SECONDS set daemon connection timeout in seconds
	Contimeout int
	// IgnoreTimes don't skip files that match size and time
	IgnoreTimes bool
	// SizeOnly skip files that match in size
	SizeOnly bool
	// ModifyWindow modify-window=NUM compare mod-times with reduced accuracy
	ModifyWindow bool
	// TempDir temp-dir=DIR create temporary files in directory DIR
	TempDir string
	// Fuzzy find similar file for basis if no dest file
	Fuzzy bool
	// CompareDest compare-dest=DIR also compare received files relative to DIR
	CompareDest string
	// CopyDest copy-dest=DIR ... and include copies of unchanged files
	CopyDest string
	// LinkDest link-dest=DIR hardlink to files in DIR when unchanged
	LinkDest string
	// Compress file data during the transfer
	Compress bool
	// CompressLevel explicitly set compression level
	CompressLevel int
	// SkipCompress skip-compress=LIST skip compressing files with suffix in LIST
	SkipCompress []string
	// CVSExclude auto-ignore files in the same way CVS does
	CVSExclude bool
	// Stats give some file-transfer stats
	Stats bool
	// HumanReadable output numbers in a human-readable format
	HumanReadable bool
	// Progress show progress during transfer
	Progress bool
	// Read daemon-access password from FILE
	PasswordFile string
	// limit socket I/O bandwidth
	BandwidthLimit int
	// Info
	Info string
	// Exclude --exclude="", exclude remote paths.
	Exclude []string
	// Include --include="", include remote paths.
	Include []string
	// Filter --filter="", include filter rule.
	Filter string
	// Chown --chown="", chown on receipt.
	Chown string

	// ipv4
	IPv4 bool
	// ipv6
	IPv6 bool

	//out-format
	OutFormat bool
}

// StdoutPipe returns a pipe that will be connected to the command's
// standard output when the command starts.
func (r Rsync) StdoutPipe() (io.ReadCloser, error) {
	return r.cmd.StdoutPipe()
}

// StderrPipe returns a pipe that will be connected to the command's
// standard error when the command starts.
func (r Rsync) StderrPipe() (io.ReadCloser, error) {
	return r.cmd.StderrPipe()
}

// Run start rsync task
func (r Rsync) Run() error {
	if !isExist(r.Destination) {
		if err := createDir(r.Destination); err != nil {
			return err
		}
	}

	if err := r.cmd.Start(); err != nil {
		return err
	}

	return r.cmd.Wait()
}

// NewRsync returns task with described options
func NewRsync(source, destination string, options RsyncOptions) *Rsync {
	arguments := append(getArguments(options), source, destination)

	binaryPath := "rsync"
	if options.RsyncBinaryPath != "" {
		binaryPath = options.RsyncBinaryPath
	}

	return &Rsync{
		Source:      source,
		Destination: destination,
		cmd:         exec.Command(binaryPath, arguments...),
	}
}

func getArguments(options RsyncOptions) []string {
	arguments := []string{}

	if options.RsyncPath != "" {
		arguments = append(arguments, "--rsync-path", options.RsyncPath)
	}

	if options.Verbose {
		arguments = append(arguments, "--verbose")
	}

	if options.Checksum {
		arguments = append(arguments, "--checksum")
	}

	if options.Quiet {
		arguments = append(arguments, "--quiet")
	}

	if options.Archive {
		arguments = append(arguments, "--archive")
	}

	if options.Recursive {
		arguments = append(arguments, "--recursive")
	}

	if options.Relative {
		arguments = append(arguments, "--relative")
	}

	if options.NoImpliedDirs {
		arguments = append(arguments, "--no-implied-dirs")
	}

	if options.Update {
		arguments = append(arguments, "--update")
	}

	if options.Inplace {
		arguments = append(arguments, "--inplace")
	}

	if options.Append {
		arguments = append(arguments, "--append")
	}

	if options.AppendVerify {
		arguments = append(arguments, "--append-verify")
	}

	if options.Dirs {
		arguments = append(arguments, "--dirs")
	}

	if options.Links {
		arguments = append(arguments, "--links")
	}

	if options.CopyLinks {
		arguments = append(arguments, "--copy-links")
	}

	if options.CopyUnsafeLinks {
		arguments = append(arguments, "--copy-unsafe-links")
	}

	if options.SafeLinks {
		arguments = append(arguments, "--safe-links")
	}

	if options.CopyDirLinks {
		arguments = append(arguments, "--copy-dir-links")
	}

	if options.KeepDirLinks {
		arguments = append(arguments, "--keep-dir-links")
	}

	if options.HardLinks {
		arguments = append(arguments, "--hard-links")
	}

	if options.Perms {
		arguments = append(arguments, "--perms")
	}

	if options.NoPerms {
		arguments = append(arguments, "--no-perms")
	}

	if options.Executability {
		arguments = append(arguments, "--executability")
	}

	if options.ACLs {
		arguments = append(arguments, "--acls")
	}

	if options.XAttrs {
		arguments = append(arguments, "--xattrs")
	}

	if options.Owner {
		arguments = append(arguments, "--owner")
	}

	if options.NoOwner {
		arguments = append(arguments, "--no-owner")
	}

	if options.Group {
		arguments = append(arguments, "--group")
	}

	if options.NoGroup {
		arguments = append(arguments, "--no-group")
	}

	if options.Devices {
		arguments = append(arguments, "--devices")
	}

	if options.Specials {
		arguments = append(arguments, "--specials")
	}

	if options.Times {
		arguments = append(arguments, "--times")
	}

	if options.NoTimes {
		arguments = append(arguments, "--no-times")
	}

	if options.OmitDirTimes {
		arguments = append(arguments, "--omit-dir-times")
	}

	if options.Super {
		arguments = append(arguments, "--super")
	}

	if options.FakeSuper {
		arguments = append(arguments, "--fake-super")
	}

	if options.Sparse {
		arguments = append(arguments, "--sparse")
	}

	if options.DryRun {
		arguments = append(arguments, "--dry-run")
	}

	if options.WholeFile {
		arguments = append(arguments, "--whole-file")
	}

	if options.OneFileSystem {
		arguments = append(arguments, "--one-file-system")
	}

	if options.BlockSize > 0 {
		arguments = append(arguments, "--block-size", strconv.Itoa(options.BlockSize))
	}

	if options.Rsh != "" {
		arguments = append(arguments, "--rsh", options.Rsh)
	}

	if options.Existing {
		arguments = append(arguments, "--existing")
	}

	if options.IgnoreExisting {
		arguments = append(arguments, "--ignore-existing")
	}

	if options.RemoveSourceFiles {
		arguments = append(arguments, "--remove-source-files")
	}

	if options.Delete {
		arguments = append(arguments, "--delete")
	}

	if options.DeleteBefore {
		arguments = append(arguments, "--delete-before")
	}

	if options.DeleteDuring {
		arguments = append(arguments, "--delete-during")
	}

	if options.DeleteDelay {
		arguments = append(arguments, "--delete-delay")
	}

	if options.DeleteAfter {
		arguments = append(arguments, "--delete-after")
	}

	if options.DeleteExcluded {
		arguments = append(arguments, "--delete-excluded")
	}

	if options.IgnoreErrors {
		arguments = append(arguments, "--ignore-errors")
	}

	if options.Force {
		arguments = append(arguments, "--force")
	}

	if options.MaxDelete > 0 {
		arguments = append(arguments, "--max-delete", strconv.Itoa(options.MaxDelete))
	}

	if options.MaxSize > 0 {
		arguments = append(arguments, "--max-size", strconv.Itoa(options.MaxSize))
	}

	if options.MinSize > 0 {
		arguments = append(arguments, "--min-size", strconv.Itoa(options.MinSize))
	}

	if options.Partial {
		arguments = append(arguments, "--partial")
	}

	if options.PartialDir != "" {
		arguments = append(arguments, "--partial-dir", options.PartialDir)
	}

	if options.DelayUpdates {
		arguments = append(arguments, "--delay-updates")
	}

	if options.PruneEmptyDirs {
		arguments = append(arguments, "--prune-empty-dirs")
	}

	if options.NumericIDs {
		arguments = append(arguments, "--numeric-ids")
	}

	if options.Timeout > 0 {
		arguments = append(arguments, "--timeout", strconv.Itoa(options.Timeout))
	}

	if options.Contimeout > 0 {
		arguments = append(arguments, "--contimeout", strconv.Itoa(options.Contimeout))
	}

	if options.IgnoreTimes {
		arguments = append(arguments, "--ignore-times")
	}

	if options.SizeOnly {
		arguments = append(arguments, "--size-only")
	}

	if options.ModifyWindow {
		arguments = append(arguments, "--modify-window")
	}

	if options.TempDir != "" {
		arguments = append(arguments, "--temp-dir", options.TempDir)
	}

	if options.Fuzzy {
		arguments = append(arguments, "--fuzzy")
	}

	if options.CompareDest != "" {
		arguments = append(arguments, "--compare-dest", options.CompareDest)
	}

	if options.CopyDest != "" {
		arguments = append(arguments, "--copy-dest", options.CopyDest)
	}

	if options.LinkDest != "" {
		arguments = append(arguments, "--link-dest", options.LinkDest)
	}

	if options.Compress {
		arguments = append(arguments, "--compress")
	}

	if options.CompressLevel > 0 {
		arguments = append(arguments, "--compress-level", strconv.Itoa(options.CompressLevel))
	}

	if len(options.SkipCompress) > 0 {
		arguments = append(arguments, "--skip-compress", strings.Join(options.SkipCompress, ","))
	}

	if options.CVSExclude {
		arguments = append(arguments, "--cvs-exclude")
	}

	if options.Stats {
		arguments = append(arguments, "--stats")
	}

	if options.HumanReadable {
		arguments = append(arguments, "--human-readable")
	}

	if options.Progress {
		arguments = append(arguments, "--progress")
	}

	if options.PasswordFile != "" {
		arguments = append(arguments, "--password-file", options.PasswordFile)
	}

	if options.BandwidthLimit > 0 {
		arguments = append(arguments, "--bwlimit", strconv.Itoa(options.BandwidthLimit))
	}

	if options.IPv4 {
		arguments = append(arguments, "--ipv4")
	}

	if options.IPv6 {
		arguments = append(arguments, "--ipv6")
	}

	if options.Info != "" {
		arguments = append(arguments, "--info", options.Info)
	}

	if options.OutFormat {
		arguments = append(arguments, "--out-format=\"%n\"")
	}

	if len(options.Include) > 0 {
		for _, pattern := range options.Include {
			arguments = append(arguments, fmt.Sprintf("--include=%s", pattern))
		}
	}

	if len(options.Exclude) > 0 {
		for _, pattern := range options.Exclude {
			arguments = append(arguments, fmt.Sprintf("--exclude=%s", pattern))
		}
	}

	if options.Filter != "" {
		arguments = append(arguments, fmt.Sprintf("--filter=%s", options.Filter))
	}

	if options.Chown != "" {
		arguments = append(arguments, fmt.Sprintf("--chown=%s", options.Chown))
	}

	return arguments
}

func createDir(dir string) error {
	cmd := exec.Command("mkdir", "-p", dir)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

func isExist(p string) bool {
	stat, err := os.Stat(p)
	return os.IsExist(err) && stat.IsDir()
}
