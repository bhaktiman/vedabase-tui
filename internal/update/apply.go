package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxChecksums = 64 << 10
	maxAsset     = 100 << 20
)

// Apply downloads release tag, verifies its checksum and replaces Exe with it.
// On any failure Exe is left untouched.
func (u *Updater) Apply(ctx context.Context, tag string, progress func(done, total int64)) error {
	if !tagRe.MatchString(tag) {
		return fmt.Errorf("update: invalid release tag %q", tag)
	}
	goos, goarch := u.platform()
	asset, bin := "vedabase_"+goos+"_"+goarch+".tar.gz", "vedabase"
	if goos == "windows" {
		asset, bin = "vedabase_"+goos+"_"+goarch+".zip", "vedabase.exe"
	}
	dl := u.base() + "/" + u.Repo + "/releases/download/" + tag + "/"

	sums, err := u.get(ctx, dl+"checksums.txt", maxChecksums, nil)
	if err != nil {
		return err
	}
	want := ""
	for _, line := range strings.Split(string(sums), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[1] == asset {
			want = f[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("update: %s not listed in checksums.txt", asset)
	}
	data, err := u.get(ctx, dl+asset, maxAsset, progress)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), want) {
		return fmt.Errorf("update: checksum mismatch for %s", asset)
	}

	var content []byte
	if goos == "windows" {
		content, err = extractZip(data, bin)
	} else {
		content, err = extractTarGz(data, bin)
	}
	if err != nil {
		return err
	}
	return u.install(content, goos == "windows")
}

// install writes content next to Exe and swaps it into place.
func (u *Updater) install(content []byte, windows bool) (err error) {
	f, err := os.CreateTemp(filepath.Dir(u.Exe), ".vedabase-update-*")
	if err != nil {
		return fmt.Errorf("update: cannot write next to %s: %w", u.Exe, err)
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			os.Remove(tmp)
		}
	}()
	if err = f.Chmod(0o755); err == nil {
		_, err = f.Write(content)
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("update: writing %s: %w", tmp, err)
	}
	if !windows {
		if err = os.Rename(tmp, u.Exe); err != nil {
			return fmt.Errorf("update: replacing %s: %w", u.Exe, err)
		}
		return nil
	}
	old := u.Exe + ".old"
	_ = os.Remove(old)
	if err = os.Rename(u.Exe, old); err != nil {
		return fmt.Errorf("update: moving %s aside: %w", u.Exe, err)
	}
	if err = os.Rename(tmp, u.Exe); err != nil {
		_ = os.Rename(old, u.Exe)
		return fmt.Errorf("update: replacing %s: %w", u.Exe, err)
	}
	return nil
}
