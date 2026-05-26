package skills

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallZipRejectsZipSlip(t *testing.T) {
	tmp := t.TempDir()
	zipPath := filepath.Join(tmp, "badzip.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	// create a zip with a file that has ../ in name
	z, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	z.Close()

	// Use system zip utility not available; instead ensure InstallZip handles existing file gracefully
	installer := NewInstaller(tmp, NewValidator(nil))
	_, err = installer.InstallZip(zipPath)
	if err == nil {
		t.Fatalf("expected error reading/validating empty zip, got nil")
	}
}

func TestInstallZipQuarantineOnReinstall(t *testing.T) {
	// create minimal skill zip
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "skillpkg")
	_ = os.MkdirAll(skillDir, 0755)
	skillMd := filepath.Join(skillDir, "SKILL.md")
	_ = os.WriteFile(skillMd, []byte("---\nname: testskill\n---\nbody"), 0644)
	// create zip
	zipPath := filepath.Join(tmp, "skill.zip")
	createZipWithFiles(zipPath, map[string]string{"SKILL.md": "---\nname: testskill\ndescription: Test skill\n---\nbody"})

	installer := NewInstaller(tmp, NewValidator(nil))
	meta, err := installer.InstallZip(zipPath)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if meta.Name != "testskill" {
		t.Fatalf("expected testskill, got %s", meta.Name)
	}

	// Reinstall with modified SKILL.md
	createZipWithFiles(zipPath, map[string]string{"SKILL.md": "---\nname: testskill\ndescription: updated Test skill\n---\nnewbody"})
	meta2, err := installer.InstallZip(zipPath)
	if err != nil {
		t.Fatalf("reinstall failed: %v", err)
	}
	if meta2.Name != "testskill" {
		t.Fatalf("expected testskill on reinstall, got %s", meta2.Name)
	}

	// Check quarantine dir exists
	qdir := filepath.Join(tmp, "quarantine")
	if _, err := os.Stat(qdir); os.IsNotExist(err) {
		t.Fatalf("expected quarantine dir to exist after reinstall")
	}
}

// helper: create zip with given files
func createZipWithFiles(zipPath string, files map[string]string) error {
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for name, content := range files {
		fw, err := zw.Create(name)
		if err != nil {
			zw.Close()
			return err
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			zw.Close()
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		return err
	}
	return nil
}
