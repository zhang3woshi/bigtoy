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
	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		return fmt.Errorf("ensure backup directory: %w", err)
	}

	now := time.Now()
	fileName := fmt.Sprintf("%s%s_%09d%s", backupFilePrefix, now.Format("20060102_150405"), now.Nanosecond(), backupFileExt)
	archivePath := filepath.Join(s.backupDir, fileName)
	tempArchivePath := archivePath + ".tmp"

	if err := s.writeBackupZip(tempArchivePath); err != nil {
		_ = os.Remove(tempArchivePath)
		return err
	}

	if err := os.Rename(tempArchivePath, archivePath); err != nil {
		_ = os.Remove(tempArchivePath)
		return fmt.Errorf("finalize backup archive: %w", err)
	}

	if err := s.pruneBackups(); err != nil {
		return fmt.Errorf("cleanup old backups: %w", err)
	}

	log.Printf("[backup] created backup archive: %s", archivePath)
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
