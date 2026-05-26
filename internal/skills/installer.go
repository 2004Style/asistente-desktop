package skills

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Installer struct {
	skillsDir string
	validator *Validator
}

func NewInstaller(skillsDir string, validator *Validator) *Installer {
	return &Installer{
		skillsDir: skillsDir,
		validator: validator,
	}
}

func (inst *Installer) InstallZip(zipPath string) (*SkillMetadata, error) {
	// 1. Validar tamaño del ZIP
	zipInfo, err := os.Stat(zipPath)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir el archivo ZIP: %w", err)
	}
	if zipInfo.Size() > 10*1024*1024 { // Máximo 10 MB
		return nil, fmt.Errorf("el archivo ZIP excede el tamaño máximo permitido (10MB)")
	}

	// 2. Crear staging temporal
	stagingDir, err := os.MkdirTemp("", "rbot-skill-staging")
	if err != nil {
		return nil, fmt.Errorf("error al crear directorio temporal de staging: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("error leyendo archivo ZIP: %w", err)
	}
	defer r.Close()

	if len(r.File) > 100 { // Máximo 100 archivos
		return nil, fmt.Errorf("el archivo ZIP contiene demasiados archivos (máximo 100)")
	}

	hasSkillMd := false
	var skillMdPath string

	hasher := sha256.New()

	for _, f := range r.File {
		// Control de Zip Slip
		cleanedPath := filepath.Clean(f.Name)
		destPath := filepath.Join(stagingDir, cleanedPath)

		if !strings.HasPrefix(destPath, filepath.Clean(stagingDir)+string(filepath.Separator)) {
			return nil, fmt.Errorf("intento de evasión de directorio detectado (Zip Slip) en: %s", f.Name)
		}

		// Bloquear enlaces simbólicos
		if f.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("los enlaces simbólicos no están permitidos en las habilidades por seguridad: %s", f.Name)
		}

		// Bloquear ejecutables
		if f.Mode()&0111 != 0 {
			return nil, fmt.Errorf("los archivos ejecutables no están permitidos en las habilidades por seguridad: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(destPath, 0755)
			continue
		}

		// Crear directorio padre si no existe
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return nil, err
		}

		outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return nil, fmt.Errorf("error al extraer archivo %s: %w", f.Name, err)
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return nil, err
		}

		// Copiar y calcular hash simultáneamente
		w := io.MultiWriter(outFile, hasher)
		_, err = io.Copy(w, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return nil, fmt.Errorf("error al copiar archivo %s: %w", f.Name, err)
		}

		if strings.HasSuffix(strings.ToLower(cleanedPath), "skill.md") {
			hasSkillMd = true
			skillMdPath = destPath
		}
	}

	if !hasSkillMd {
		return nil, fmt.Errorf("el paquete de la skill no contiene el archivo SKILL.md obligatorio")
	}

	// 3. Parsear y Validar la metadata de la skill en staging
	meta, _, err := ParseSkillMd(skillMdPath)
	if err != nil {
		return nil, fmt.Errorf("error al parsear SKILL.md en staging: %w", err)
	}

	if err := inst.validator.Validate(meta); err != nil {
		return nil, fmt.Errorf("la skill en staging no es válida: %w", err)
	}

	// 4. Mover de staging a la carpeta de destino final
	finalDestDir := filepath.Join(inst.skillsDir, meta.Name)
	_ = os.RemoveAll(finalDestDir) // Limpiar previo si existe

	if err := os.MkdirAll(finalDestDir, 0755); err != nil {
		return nil, fmt.Errorf("error al crear directorio de destino para la skill: %w", err)
	}

	// Copiar archivos de staging a la ruta final
	if err := copyDirectory(stagingDir, finalDestDir); err != nil {
		return nil, fmt.Errorf("error al mover skill al directorio final: %w", err)
	}

	// Establecer hash final calculado y estado inicial disabled
	meta.Status = "disabled"

	return meta, nil
}

func copyDirectory(scrDir, dest string) error {
	entries, err := os.ReadDir(scrDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(scrDir, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return err
			}
			if err := copyDirectory(sourcePath, destPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(sourcePath, destPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Sync()
}

func CalculateDirHash(dirPath string) (string, error) {
	hasher := sha256.New()
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(hasher, f); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}
