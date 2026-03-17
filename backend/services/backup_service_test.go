package services

import (
	"archive/zip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"bigtoy/backend/models"
)

func managedBackupFiles(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read backup dir: %v", err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if isManagedBackupArchive(entry.Name()) {
			files = append(files, entry.Name())
		}
	}
	slices.Sort(files)
	return files
}

func TestNewBackupServiceValidation(t *testing.T) {
	tmp := t.TempDir()

	valid := BackupServiceConfig{
		DBPath:     filepath.Join(tmp, "data", "models.db"),
		ImagesRoot: filepath.Join(tmp, "images"),
		BackupDir:  filepath.Join(tmp, "backup"),
		Interval:   time.Minute,
		MaxBackups: 2,
	}

	if _, err := NewBackupService(valid); err != nil {
		t.Fatalf("expected valid backup service config, got: %v", err)
	}

	if _, err := NewBackupService(BackupServiceConfig{
		ImagesRoot: valid.ImagesRoot,
		BackupDir:  valid.BackupDir,
		Interval:   valid.Interval,
	}); err == nil {
		t.Fatal("expected error when DB path is empty")
	}

	if _, err := NewBackupService(BackupServiceConfig{
		DBPath:    valid.DBPath,
		BackupDir: valid.BackupDir,
		Interval:  valid.Interval,
	}); err == nil {
		t.Fatal("expected error when images root is empty")
	}

	if _, err := NewBackupService(BackupServiceConfig{
		DBPath:     valid.DBPath,
		ImagesRoot: valid.ImagesRoot,
		Interval:   valid.Interval,
	}); err == nil {
		t.Fatal("expected error when backup directory is empty")
	}

	if _, err := NewBackupService(BackupServiceConfig{
		DBPath:     valid.DBPath,
		ImagesRoot: valid.ImagesRoot,
		BackupDir:  valid.BackupDir,
	}); err == nil {
		t.Fatal("expected error when interval is invalid")
	}
}

func TestCreateBackupZipIncludesExpectedEntries(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data", "models.db")
	imagesRoot := filepath.Join(tmp, "images")
	backupDir := filepath.Join(tmp, "backup")

	if err := os.MkdirAll(filepath.Join(tmp, "data"), 0o755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(imagesRoot, "1"), 0o755); err != nil {
		t.Fatalf("create images dir: %v", err)
	}
	if err := os.WriteFile(dbPath, []byte("sqlite-data"), 0o644); err != nil {
		t.Fatalf("write db file: %v", err)
	}
	if err := os.WriteFile(dbPath+"-wal", []byte("wal-data"), 0o644); err != nil {
		t.Fatalf("write wal file: %v", err)
	}
	if err := os.WriteFile(dbPath+"-shm", []byte("shm-data"), 0o644); err != nil {
		t.Fatalf("write shm file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(imagesRoot, "1", "cover.png"), []byte("img-data"), 0o644); err != nil {
		t.Fatalf("write image file: %v", err)
	}

	service, err := NewBackupService(BackupServiceConfig{
		DBPath:     dbPath,
		ImagesRoot: imagesRoot,
		BackupDir:  backupDir,
		Interval:   time.Minute,
		MaxBackups: 3,
	})
	if err != nil {
		t.Fatalf("new backup service: %v", err)
	}

	if err := service.CreateBackup(); err != nil {
		t.Fatalf("create backup: %v", err)
	}

	files := managedBackupFiles(t, backupDir)
	if len(files) != 1 {
		t.Fatalf("expected one backup archive, got %d", len(files))
	}

	archive, err := zip.OpenReader(filepath.Join(backupDir, files[0]))
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer archive.Close()

	names := make([]string, 0, len(archive.File))
	for _, file := range archive.File {
		names = append(names, file.Name)
	}
	slices.Sort(names)

	required := []string{
		"db/models.db",
		"db/models.db-shm",
		"db/models.db-wal",
		"images/1/cover.png",
	}
	for _, name := range required {
		if !slices.Contains(names, name) {
			t.Fatalf("missing archive entry %q in %#v", name, names)
		}
	}
}

func TestCreateBackupArchiveReturnsCreatedFilePath(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data", "models.db")
	imagesRoot := filepath.Join(tmp, "images")
	backupDir := filepath.Join(tmp, "backup")

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("create db parent dir: %v", err)
	}
	if err := os.MkdirAll(imagesRoot, 0o755); err != nil {
		t.Fatalf("create images dir: %v", err)
	}
	if err := os.WriteFile(dbPath, []byte("sqlite-data"), 0o644); err != nil {
		t.Fatalf("write db file: %v", err)
	}

	service, err := NewBackupService(BackupServiceConfig{
		DBPath:     dbPath,
		ImagesRoot: imagesRoot,
		BackupDir:  backupDir,
		Interval:   time.Minute,
		MaxBackups: 3,
	})
	if err != nil {
		t.Fatalf("new backup service: %v", err)
	}

	archivePath, err := service.CreateBackupArchive()
	if err != nil {
		t.Fatalf("create backup archive: %v", err)
	}
	if strings.TrimSpace(archivePath) == "" {
		t.Fatal("expected non-empty archive path")
	}
	if filepath.Ext(archivePath) != ".zip" {
		t.Fatalf("expected zip archive path, got %s", archivePath)
	}
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("expected created archive file to exist: %v", err)
	}
}

func TestCreateBackupPrunesOldArchives(t *testing.T) {
	tmp, err := os.MkdirTemp("", "bigtoy-backup-prune-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tmp)
	})

	dbPath := filepath.Join(tmp, "models.db")
	imagesRoot := filepath.Join(tmp, "images")
	backupDir := filepath.Join(tmp, "backup")

	if err := os.MkdirAll(imagesRoot, 0o755); err != nil {
		t.Fatalf("create images root: %v", err)
	}
	if err := os.WriteFile(dbPath, []byte("sqlite-data"), 0o644); err != nil {
		t.Fatalf("write db file: %v", err)
	}

	service, err := NewBackupService(BackupServiceConfig{
		DBPath:     dbPath,
		ImagesRoot: imagesRoot,
		BackupDir:  backupDir,
		Interval:   time.Minute,
		MaxBackups: 2,
	})
	if err != nil {
		t.Fatalf("new backup service: %v", err)
	}

	for i := 0; i < 4; i++ {
		if err := service.CreateBackup(); err != nil {
			t.Fatalf("create backup %d: %v", i+1, err)
		}
		time.Sleep(5 * time.Millisecond)
	}

	files := managedBackupFiles(t, backupDir)
	if len(files) != 2 {
		t.Fatalf("expected exactly 2 retained backups, got %d (%#v)", len(files), files)
	}
}

func TestBackupServiceStartAndStop(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "models.db")
	imagesRoot := filepath.Join(tmp, "images")
	backupDir := filepath.Join(tmp, "backup")

	if err := os.MkdirAll(imagesRoot, 0o755); err != nil {
		t.Fatalf("create images root: %v", err)
	}
	if err := os.WriteFile(dbPath, []byte("sqlite-data"), 0o644); err != nil {
		t.Fatalf("write db file: %v", err)
	}

	service, err := NewBackupService(BackupServiceConfig{
		DBPath:     dbPath,
		ImagesRoot: imagesRoot,
		BackupDir:  backupDir,
		Interval:   20 * time.Millisecond,
		MaxBackups: 3,
	})
	if err != nil {
		t.Fatalf("new backup service: %v", err)
	}

	service.Start()
	time.Sleep(60 * time.Millisecond)
	service.Stop()
	service.Stop()

	files := managedBackupFiles(t, backupDir)
	if len(files) == 0 {
		t.Fatal("expected backup files to be created by scheduler loop")
	}
}

func TestIsManagedBackupArchive(t *testing.T) {
	cases := map[string]bool{
		"backup_20260311_123456_000000001.zip": true,
		" BACKUP_abc.zip ":                     true,
		"backup_abc.tmp":                       false,
		"random.zip":                           false,
		"":                                     false,
	}

	for name, expected := range cases {
		if actual := isManagedBackupArchive(name); actual != expected {
			t.Fatalf("isManagedBackupArchive(%q)=%v, expected %v", name, actual, expected)
		}
	}
}

func TestAddFileToZipRejectsMissingSource(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "archive.zip")
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive file: %v", err)
	}
	zipWriter := zip.NewWriter(archiveFile)

	err = addFileToZip(zipWriter, filepath.Join(tmp, "missing.txt"), "missing.txt")
	if err == nil || !strings.Contains(err.Error(), "open source file") {
		t.Fatalf("expected open source file error, got: %v", err)
	}

	_ = zipWriter.Close()
	_ = archiveFile.Close()
}

func TestCreateBackupFailsWhenDatabaseIsMissing(t *testing.T) {
	tmp := t.TempDir()
	service, err := NewBackupService(BackupServiceConfig{
		DBPath:     filepath.Join(tmp, "missing.db"),
		ImagesRoot: filepath.Join(tmp, "images"),
		BackupDir:  filepath.Join(tmp, "backup"),
		Interval:   time.Minute,
		MaxBackups: 3,
	})
	if err != nil {
		t.Fatalf("new backup service: %v", err)
	}

	if err := service.CreateBackup(); err == nil {
		t.Fatal("expected error when database file is missing")
	}
}

func TestAddDirectoryToZipReturnsErrorForMissingDirectory(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "archive.zip")
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive file: %v", err)
	}
	zipWriter := zip.NewWriter(archiveFile)

	service := &BackupService{}
	err = service.addDirectoryToZip(zipWriter, filepath.Join(tmp, "missing-dir"), "images")
	if err == nil {
		t.Fatal("expected missing directory walk error")
	}

	_ = zipWriter.Close()
	_ = archiveFile.Close()
}

func TestAddDatabaseCompanionSkipsDirectory(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "archive.zip")
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive file: %v", err)
	}
	zipWriter := zip.NewWriter(archiveFile)

	companionDir := filepath.Join(tmp, "models.db-wal")
	if err := os.MkdirAll(companionDir, 0o755); err != nil {
		t.Fatalf("create companion dir: %v", err)
	}

	service := &BackupService{}
	if err := service.addDatabaseCompanionFileToZip(zipWriter, companionDir); err != nil {
		t.Fatalf("expected companion directory to be skipped, got: %v", err)
	}

	_ = zipWriter.Close()
	_ = archiveFile.Close()
}

func TestAddFileToZipSkipsDirectory(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "archive.zip")
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive file: %v", err)
	}
	zipWriter := zip.NewWriter(archiveFile)

	dirPath := filepath.Join(tmp, "folder")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}

	if err := addFileToZip(zipWriter, dirPath, "folder"); err != nil {
		t.Fatalf("expected directory source to be skipped, got: %v", err)
	}

	_ = zipWriter.Close()
	_ = archiveFile.Close()
}

func TestWriteBackupZipFailsWhenImagesDirectoryIsMissing(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "models.db")
	if err := os.WriteFile(dbPath, []byte("sqlite-data"), 0o644); err != nil {
		t.Fatalf("write db file: %v", err)
	}

	service := &BackupService{
		dbPath:     dbPath,
		imagesRoot: filepath.Join(tmp, "missing-images"),
	}

	archivePath := filepath.Join(tmp, "archive.zip")
	if err := service.writeBackupZip(archivePath); err == nil {
		t.Fatal("expected writeBackupZip to fail when images directory is missing")
	}
}

func TestRestoreBackupArchiveRestoresDatabaseAndImages(t *testing.T) {
	tmp := t.TempDir()

	sourceRoot := filepath.Join(tmp, "source")
	sourceDB := filepath.Join(sourceRoot, "models.db")
	sourceImages := filepath.Join(sourceRoot, "images")
	sourceBackupDir := filepath.Join(sourceRoot, "backup")

	sourceStore, err := NewModelStore(sourceDB, sourceImages, "")
	if err != nil {
		t.Fatalf("create source model store: %v", err)
	}
	t.Cleanup(func() { _ = sourceStore.Close() })

	if _, err := sourceStore.Add(models.CreateModelRequest{Name: "Source Model", Year: 2024}); err != nil {
		t.Fatalf("insert source model: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceImages, "source-only"), 0o755); err != nil {
		t.Fatalf("create source image dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceImages, "source-only", "cover.png"), []byte("source"), 0o644); err != nil {
		t.Fatalf("write source image: %v", err)
	}

	sourceBackupService, err := NewBackupService(BackupServiceConfig{
		DBPath:     sourceDB,
		ImagesRoot: sourceImages,
		BackupDir:  sourceBackupDir,
		Interval:   time.Minute,
		MaxBackups: 3,
	})
	if err != nil {
		t.Fatalf("create source backup service: %v", err)
	}
	archivePath, err := sourceBackupService.CreateBackupArchive()
	if err != nil {
		t.Fatalf("create source backup archive: %v", err)
	}

	targetRoot := filepath.Join(tmp, "target")
	targetDB := filepath.Join(targetRoot, "models.db")
	targetImages := filepath.Join(targetRoot, "images")
	targetStore, err := NewModelStore(targetDB, targetImages, "")
	if err != nil {
		t.Fatalf("create target model store: %v", err)
	}
	if _, err := targetStore.Add(models.CreateModelRequest{Name: "Target Model", Year: 2001}); err != nil {
		t.Fatalf("insert target model: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(targetImages, "target-only"), 0o755); err != nil {
		t.Fatalf("create target image dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetImages, "target-only", "cover.png"), []byte("target"), 0o644); err != nil {
		t.Fatalf("write target image: %v", err)
	}
	if err := targetStore.Close(); err != nil {
		t.Fatalf("close target model store: %v", err)
	}

	restoreService, err := NewBackupService(BackupServiceConfig{
		DBPath:     targetDB,
		ImagesRoot: targetImages,
		BackupDir:  filepath.Join(targetRoot, "backup"),
		Interval:   time.Minute,
		MaxBackups: 3,
	})
	if err != nil {
		t.Fatalf("create restore backup service: %v", err)
	}

	if err := restoreService.RestoreBackupArchive(archivePath); err != nil {
		t.Fatalf("restore backup archive: %v", err)
	}

	restoredStore, err := NewModelStore(targetDB, targetImages, "")
	if err != nil {
		t.Fatalf("reopen restored model store: %v", err)
	}
	defer restoredStore.Close()

	items := restoredStore.List()
	if len(items) != 1 {
		t.Fatalf("expected 1 restored model, got %d", len(items))
	}
	if items[0].Name != "Source Model" {
		t.Fatalf("expected restored source model, got %#v", items[0])
	}

	if _, err := os.Stat(filepath.Join(targetImages, "source-only", "cover.png")); err != nil {
		t.Fatalf("expected source image file to exist after restore: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetImages, "target-only", "cover.png")); !os.IsNotExist(err) {
		t.Fatalf("expected target-only image to be replaced, got err=%v", err)
	}
}

func TestRestoreBackupArchiveRejectsInvalidArchive(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "models.db")
	imagesRoot := filepath.Join(tmp, "images")
	backupDir := filepath.Join(tmp, "backup")

	if err := os.MkdirAll(imagesRoot, 0o755); err != nil {
		t.Fatalf("create images root: %v", err)
	}
	if err := os.WriteFile(dbPath, []byte("db"), 0o644); err != nil {
		t.Fatalf("write db file: %v", err)
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("create backup dir: %v", err)
	}

	badArchivePath := filepath.Join(backupDir, "invalid.zip")
	if err := os.WriteFile(badArchivePath, []byte("not-a-zip"), 0o644); err != nil {
		t.Fatalf("write invalid archive: %v", err)
	}

	service, err := NewBackupService(BackupServiceConfig{
		DBPath:     dbPath,
		ImagesRoot: imagesRoot,
		BackupDir:  backupDir,
		Interval:   time.Minute,
		MaxBackups: 3,
	})
	if err != nil {
		t.Fatalf("create backup service: %v", err)
	}

	if err := service.RestoreBackupArchive(badArchivePath); err == nil {
		t.Fatal("expected restore to fail for invalid archive file")
	}
}

func TestReplaceDirectoryInPlace(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "source")
	targetDir := filepath.Join(root, "target")

	if err := os.MkdirAll(filepath.Join(sourceDir, "new"), 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(targetDir, "old"), 0o755); err != nil {
		t.Fatalf("create target dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(sourceDir, "new", "a.txt"), []byte("new"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "old", "b.txt"), []byte("old"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	if err := replaceDirectoryInPlace(sourceDir, targetDir); err != nil {
		t.Fatalf("replace directory in place: %v", err)
	}

	if _, err := os.Stat(filepath.Join(targetDir, "new", "a.txt")); err != nil {
		t.Fatalf("expected new file after in-place replace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "old", "b.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected stale file to be removed, got err=%v", err)
	}
}
