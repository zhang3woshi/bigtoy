package services

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	backupFilePrefix = "backup_"
	backupFileExt    = ".zip"
)

type BackupServiceConfig struct {
	DBPath     string
	ImagesRoot string
	BackupDir  string
	Interval   time.Duration
	MaxBackups int
}

type BackupService struct {
	dbPath     string
	imagesRoot string
	backupDir  string
	interval   time.Duration
	maxBackups int

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
	doneCh    chan struct{}
}

type backupArchiveEntry struct {
	name    string
	full    string
	modTime time.Time
}

func NewBackupService(config BackupServiceConfig) (*BackupService, error) {
	dbPath := strings.TrimSpace(config.DBPath)
	if dbPath == "" {
		return nil, errors.New("backup db path is required")
	}

	imagesRoot := strings.TrimSpace(config.ImagesRoot)
	if imagesRoot == "" {
		return nil, errors.New("backup images root path is required")
	}
	if err := os.MkdirAll(imagesRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create images root: %w", err)
	}

	backupDir := strings.TrimSpace(config.BackupDir)
	if backupDir == "" {
		return nil, errors.New("backup directory is required")
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return nil, fmt.Errorf("create backup directory: %w", err)
	}

	interval := config.Interval
	if interval <= 0 {
		return nil, errors.New("backup interval must be greater than 0")
	}

	maxBackups := config.MaxBackups
	if maxBackups <= 0 {
		maxBackups = 3
	}

	return &BackupService{
		dbPath:     dbPath,
		imagesRoot: imagesRoot,
		backupDir:  backupDir,
		interval:   interval,
		maxBackups: maxBackups,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}, nil
}

func (s *BackupService) Start() {
	s.startOnce.Do(func() {
		go s.loop()
	})
}

func (s *BackupService) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	<-s.doneCh
}

func (s *BackupService) loop() {
	defer close(s.doneCh)

	if err := s.CreateBackup(); err != nil {
		log.Printf("[backup] initial backup failed: %v", err)
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			if err := s.CreateBackup(); err != nil {
				log.Printf("[backup] scheduled backup failed: %v", err)
			}
		}
	}
}

func (s *BackupService) CreateBackup() error {
	_, err := s.CreateBackupArchive()
	return err
}

func (s *BackupService) CreateBackupArchive() (string, error) {
	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		return "", fmt.Errorf("ensure backup directory: %w", err)
	}

	now := time.Now()
	fileName := fmt.Sprintf("%s%s_%09d%s", backupFilePrefix, now.Format("20060102_150405"), now.Nanosecond(), backupFileExt)
	archivePath := filepath.Join(s.backupDir, fileName)
	tempArchivePath := archivePath + ".tmp"

	if err := s.writeBackupZip(tempArchivePath); err != nil {
		_ = os.Remove(tempArchivePath)
		return "", err
	}

	if err := os.Rename(tempArchivePath, archivePath); err != nil {
		_ = os.Remove(tempArchivePath)
		return "", fmt.Errorf("finalize backup archive: %w", err)
	}

	if err := s.pruneBackups(); err != nil {
		return "", fmt.Errorf("cleanup old backups: %w", err)
	}

	log.Printf("[backup] created backup archive: %s", archivePath)
	return archivePath, nil
}

func (s *BackupService) RestoreBackupArchive(archivePath string) error {
	archivePath = strings.TrimSpace(archivePath)
	if archivePath == "" {
		return errors.New("backup archive path is required")
	}

	archiveInfo, err := os.Stat(archivePath)
	if err != nil {
		return fmt.Errorf("read backup archive info: %w", err)
	}
	if archiveInfo.IsDir() {
		return errors.New("backup archive path must be a file")
	}

	restoreRoot, err := os.MkdirTemp(s.backupDir, "restore_*")
	if err != nil {
		return fmt.Errorf("create restore workspace: %w", err)
	}
	defer os.RemoveAll(restoreRoot)

	if err := extractBackupZip(archivePath, restoreRoot); err != nil {
		return err
	}

	dbSourcePath, err := findDatabaseSourcePath(restoreRoot, filepath.Base(s.dbPath))
	if err != nil {
		return err
	}

	imagesSourcePath := filepath.Join(restoreRoot, "images")
	if info, statErr := os.Stat(imagesSourcePath); statErr != nil || !info.IsDir() {
		if errors.Is(statErr, os.ErrNotExist) {
			if err := os.MkdirAll(imagesSourcePath, 0o755); err != nil {
				return fmt.Errorf("create empty images source: %w", err)
			}
		} else if statErr != nil {
			return fmt.Errorf("read images source info: %w", statErr)
		} else {
			return errors.New("images source path is not a directory")
		}
	}

	if err := replaceDatabaseFile(dbSourcePath, s.dbPath); err != nil {
		return err
	}
	if err := replaceDirectory(imagesSourcePath, s.imagesRoot); err != nil {
		return err
	}

	_ = os.Remove(s.dbPath + "-wal")
	_ = os.Remove(s.dbPath + "-shm")
	return nil
}

func (s *BackupService) writeBackupZip(archivePath string) error {
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("create backup archive: %w", err)
	}

	zipWriter := zip.NewWriter(archiveFile)
	if err := s.addDatabaseFileToZip(zipWriter); err != nil {
		_ = zipWriter.Close()
		_ = archiveFile.Close()
		return err
	}
	if err := s.addDirectoryToZip(zipWriter, s.imagesRoot, "images"); err != nil {
		_ = zipWriter.Close()
		_ = archiveFile.Close()
		return err
	}

	if err := zipWriter.Close(); err != nil {
		_ = archiveFile.Close()
		return fmt.Errorf("close backup zip writer: %w", err)
	}
	if err := archiveFile.Close(); err != nil {
		return fmt.Errorf("close backup archive file: %w", err)
	}

	return nil
}

func (s *BackupService) addDatabaseFileToZip(zipWriter *zip.Writer) error {
	if err := addFileToZip(zipWriter, s.dbPath, filepath.ToSlash(filepath.Join("db", filepath.Base(s.dbPath)))); err != nil {
		return fmt.Errorf("add sqlite file to archive: %w", err)
	}
	if err := s.addDatabaseCompanionFileToZip(zipWriter, s.dbPath+"-wal"); err != nil {
		return err
	}
	if err := s.addDatabaseCompanionFileToZip(zipWriter, s.dbPath+"-shm"); err != nil {
		return err
	}
	return nil
}

func (s *BackupService) addDatabaseCompanionFileToZip(zipWriter *zip.Writer, filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read sqlite companion file info %s: %w", filePath, err)
	}
	if info.IsDir() {
		return nil
	}

	archiveName := filepath.ToSlash(filepath.Join("db", filepath.Base(filePath)))
	if err := addFileToZip(zipWriter, filePath, archiveName); err != nil {
		return fmt.Errorf("add sqlite companion file %s: %w", filePath, err)
	}
	return nil
}

func (s *BackupService) addDirectoryToZip(zipWriter *zip.Writer, directory, archiveRoot string) error {
	walkErr := filepath.WalkDir(directory, func(currentPath string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(directory, currentPath)
		if err != nil {
			return fmt.Errorf("resolve relative path: %w", err)
		}

		archiveName := filepath.ToSlash(filepath.Join(archiveRoot, relPath))
		if err := addFileToZip(zipWriter, currentPath, archiveName); err != nil {
			return err
		}
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("archive directory %s: %w", directory, walkErr)
	}
	return nil
}

func addFileToZip(zipWriter *zip.Writer, sourcePath, archiveName string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file %s: %w", sourcePath, err)
	}
	defer sourceFile.Close()

	info, err := sourceFile.Stat()
	if err != nil {
		return fmt.Errorf("read source file info %s: %w", sourcePath, err)
	}
	if info.IsDir() {
		return nil
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("build zip header for %s: %w", sourcePath, err)
	}
	header.Method = zip.Deflate
	header.Name = strings.TrimLeft(filepath.ToSlash(archiveName), "/")

	entryWriter, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create zip entry %s: %w", archiveName, err)
	}

	if _, err := io.Copy(entryWriter, sourceFile); err != nil {
		return fmt.Errorf("write zip entry %s: %w", archiveName, err)
	}
	return nil
}

func (s *BackupService) pruneBackups() error {
	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		return err
	}

	archives := make([]backupArchiveEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if !isManagedBackupArchive(name) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		archives = append(archives, backupArchiveEntry{
			name:    name,
			full:    filepath.Join(s.backupDir, name),
			modTime: info.ModTime(),
		})
	}

	if len(archives) <= s.maxBackups {
		return nil
	}

	sort.Slice(archives, func(i, j int) bool {
		if archives[i].modTime.Equal(archives[j].modTime) {
			return archives[i].name < archives[j].name
		}
		return archives[i].modTime.Before(archives[j].modTime)
	})

	excess := len(archives) - s.maxBackups
	for i := 0; i < excess; i++ {
		candidate := archives[i]
		if err := os.Remove(candidate.full); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove old backup %s: %w", candidate.full, err)
		}
		log.Printf("[backup] removed old backup archive: %s", candidate.full)
	}

	return nil
}

func isManagedBackupArchive(fileName string) bool {
	name := strings.ToLower(strings.TrimSpace(fileName))
	return strings.HasPrefix(name, backupFilePrefix) && strings.HasSuffix(name, backupFileExt)
}

func extractBackupZip(archivePath, destination string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open backup archive: %w", err)
	}
	defer reader.Close()

	cleanDestination := filepath.Clean(destination)
	if err := os.MkdirAll(cleanDestination, 0o755); err != nil {
		return fmt.Errorf("ensure restore destination: %w", err)
	}

	prefix := cleanDestination + string(filepath.Separator)
	for _, entry := range reader.File {
		normalizedName := filepath.Clean(filepath.FromSlash(strings.TrimSpace(entry.Name)))
		if normalizedName == "." || normalizedName == "" {
			continue
		}
		if filepath.IsAbs(normalizedName) || strings.HasPrefix(normalizedName, "..") {
			return fmt.Errorf("invalid backup archive entry: %s", entry.Name)
		}

		targetPath := filepath.Join(cleanDestination, normalizedName)
		targetPath = filepath.Clean(targetPath)
		if targetPath != cleanDestination && !strings.HasPrefix(targetPath, prefix) {
			return fmt.Errorf("invalid backup archive entry: %s", entry.Name)
		}

		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create restore directory %s: %w", targetPath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create restore file parent %s: %w", targetPath, err)
		}

		source, err := entry.Open()
		if err != nil {
			return fmt.Errorf("open archive entry %s: %w", entry.Name, err)
		}

		target, err := os.Create(targetPath)
		if err != nil {
			source.Close()
			return fmt.Errorf("create restore file %s: %w", targetPath, err)
		}

		if _, err := io.Copy(target, source); err != nil {
			target.Close()
			source.Close()
			return fmt.Errorf("write restore file %s: %w", targetPath, err)
		}

		if err := target.Close(); err != nil {
			source.Close()
			return fmt.Errorf("close restore file %s: %w", targetPath, err)
		}
		if err := source.Close(); err != nil {
			return fmt.Errorf("close archive entry %s: %w", entry.Name, err)
		}
	}

	return nil
}

func findDatabaseSourcePath(restoreRoot, expectedBaseName string) (string, error) {
	dbRoot := filepath.Join(restoreRoot, "db")
	candidate := filepath.Join(dbRoot, strings.TrimSpace(expectedBaseName))
	info, err := os.Stat(candidate)
	if err == nil && !info.IsDir() {
		return candidate, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read database backup info: %w", err)
	}

	entries, err := os.ReadDir(dbRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errors.New("backup archive missing db directory")
		}
		return "", fmt.Errorf("read backup db directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(entry.Name()))
		if strings.HasSuffix(name, ".db") {
			return filepath.Join(dbRoot, entry.Name()), nil
		}
	}

	return "", errors.New("backup archive missing database file")
}

func replaceDatabaseFile(sourcePath, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("ensure target db directory: %w", err)
	}

	tempPath := targetPath + ".restore_tmp"
	if err := copyFile(sourcePath, tempPath); err != nil {
		return fmt.Errorf("prepare restored database file: %w", err)
	}

	if err := os.Remove(targetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(tempPath)
		return fmt.Errorf("remove existing database file: %w", err)
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace database file: %w", err)
	}
	return nil
}

func replaceDirectory(sourceDir, targetDir string) error {
	parentDir := filepath.Dir(targetDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("ensure target parent directory: %w", err)
	}

	stagingDir := filepath.Join(parentDir, fmt.Sprintf(".restore_images_%d", time.Now().UnixNano()))
	if err := copyDirectory(sourceDir, stagingDir); err != nil {
		return fmt.Errorf("prepare restored images directory: %w", err)
	}

	backupDir := filepath.Join(parentDir, fmt.Sprintf(".restore_images_backup_%d", time.Now().UnixNano()))
	hasOriginal := false
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.Rename(targetDir, backupDir); err != nil {
			_ = os.RemoveAll(stagingDir)
			return fmt.Errorf("backup existing images directory: %w", err)
		}
		hasOriginal = true
	} else if !errors.Is(err, os.ErrNotExist) {
		_ = os.RemoveAll(stagingDir)
		return fmt.Errorf("read existing images directory info: %w", err)
	}

	if err := os.Rename(stagingDir, targetDir); err != nil {
		if hasOriginal {
			_ = os.Rename(backupDir, targetDir)
		}
		_ = os.RemoveAll(stagingDir)
		return fmt.Errorf("replace images directory: %w", err)
	}

	if hasOriginal {
		_ = os.RemoveAll(backupDir)
	}
	return nil
}

func copyDirectory(sourceDir, targetDir string) error {
	sourceInfo, err := os.Stat(sourceDir)
	if err != nil {
		return fmt.Errorf("read source directory info: %w", err)
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source directory is not a directory: %s", sourceDir)
	}

	if err := os.RemoveAll(targetDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cleanup target directory: %w", err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	walkErr := filepath.WalkDir(sourceDir, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(sourceDir, currentPath)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		targetPath := filepath.Join(targetDir, relPath)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		return copyFile(currentPath, targetPath)
	})
	if walkErr != nil {
		return walkErr
	}

	return nil
}

func copyFile(sourcePath, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file %s: %w", sourcePath, err)
	}
	defer source.Close()

	info, err := source.Stat()
	if err != nil {
		return fmt.Errorf("read source file info %s: %w", sourcePath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("source file is a directory: %s", sourcePath)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create target parent %s: %w", targetPath, err)
	}

	target, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("create target file %s: %w", targetPath, err)
	}

	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		return fmt.Errorf("copy file %s -> %s: %w", sourcePath, targetPath, err)
	}
	if err := target.Close(); err != nil {
		return fmt.Errorf("close target file %s: %w", targetPath, err)
	}

	if err := os.Chmod(targetPath, info.Mode()); err != nil {
		return fmt.Errorf("set file mode for %s: %w", targetPath, err)
	}

	return nil
}
