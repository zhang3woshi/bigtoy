package services

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bigtoy/backend/models"

	_ "modernc.org/sqlite"
)

const (
	maxUploadBytes = 20 << 20
)

var ErrModelNotFound = errors.New("model not found")

type ModelStore struct {
	db             *sql.DB
	imagesRoot     string
	legacyDataPath string
}

func NewModelStore(dbPath, imagesRoot, legacyDataPath string) (*ModelStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}
	if err := os.MkdirAll(imagesRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create images directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	store := &ModelStore{
		db:             db,
		imagesRoot:     imagesRoot,
		legacyDataPath: legacyDataPath,
	}

	if err := store.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.seedFromLegacyDataIfNeeded(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *ModelStore) List() []models.CarModel {
	rows, err := s.db.Query(`
		SELECT id, name, model_code, brand, series, scale, year, color, material, condition,
		       image_url, gallery_json, notes, tags_json, created_at, updated_at
		FROM car_models
		ORDER BY id ASC
	`)
	if err != nil {
		log.Printf("list models query failed: %v", err)
		return []models.CarModel{}
	}
	defer rows.Close()

	items := make([]models.CarModel, 0)
	for rows.Next() {
		item, err := scanModel(rows)
		if err != nil {
			log.Printf("scan model row failed: %v", err)
			continue
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		log.Printf("list models iteration failed: %v", err)
	}

	return items
}

func (s *ModelStore) Add(req models.CreateModelRequest) (models.CarModel, error) {
	return s.add(req, nil, nil)
}

func (s *ModelStore) AddWithUploads(req models.CreateModelRequest, coverFile *multipart.FileHeader, galleryFiles []*multipart.FileHeader) (models.CarModel, error) {
	return s.add(req, coverFile, galleryFiles)
}

func (s *ModelStore) Update(id int64, req models.CreateModelRequest) (models.CarModel, error) {
	return s.update(id, req, nil, nil)
}

func (s *ModelStore) UpdateWithUploads(id int64, req models.CreateModelRequest, coverFile *multipart.FileHeader, galleryFiles []*multipart.FileHeader) (models.CarModel, error) {
	return s.update(id, req, coverFile, galleryFiles)
}

func (s *ModelStore) Delete(id int64) error {
	if id <= 0 {
		return errors.New("invalid model id")
	}

	result, err := s.db.Exec(`DELETE FROM car_models WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete model: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted row count: %w", err)
	}
	if rows == 0 {
		return ErrModelNotFound
	}

	modelDir := filepath.Join(s.imagesRoot, strconv.FormatInt(id, 10))
	if err := os.RemoveAll(modelDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("remove model image directory failed (id=%d): %v", id, err)
	}

	return nil
}

func (s *ModelStore) add(req models.CreateModelRequest, coverFile *multipart.FileHeader, galleryFiles []*multipart.FileHeader) (models.CarModel, error) {
	req = normalizeCreateModelRequest(req)
	if req.Name == "" {
		return models.CarModel{}, errors.New("name is required")
	}
	if req.Year < 0 {
		return models.CarModel{}, errors.New("year must be a positive number")
	}

	now := time.Now().UTC()
	timeText := now.Format(time.RFC3339Nano)
	imageURL := req.ImageURL
	gallery := normalizeURLList(req.Gallery)
	tags := normalizeTags(req.Tags)

	galleryJSON, err := marshalStringSlice(gallery)
	if err != nil {
		return models.CarModel{}, fmt.Errorf("marshal gallery: %w", err)
	}
	tagsJSON, err := marshalStringSlice(tags)
	if err != nil {
		return models.CarModel{}, fmt.Errorf("marshal tags: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return models.CarModel{}, fmt.Errorf("begin transaction: %w", err)
	}

	result, err := tx.Exec(`
		INSERT INTO car_models(
			name, model_code, brand, series, scale, year, color, material, condition,
			image_url, gallery_json, notes, tags_json, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, req.Name, req.ModelCode, req.Brand, req.Series, req.Scale, req.Year, req.Color, req.Material, req.Condition,
		imageURL, galleryJSON, req.Notes, tagsJSON, timeText, timeText)
	if err != nil {
		_ = tx.Rollback()
		return models.CarModel{}, fmt.Errorf("insert model: %w", err)
	}

	modelID, err := result.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		return models.CarModel{}, fmt.Errorf("read inserted id: %w", err)
	}

	modelDir := filepath.Join(s.imagesRoot, strconv.FormatInt(modelID, 10))
	shouldCleanupDir := false

	if coverFile != nil || len(galleryFiles) > 0 {
		if err := os.MkdirAll(modelDir, 0o755); err != nil {
			_ = tx.Rollback()
			return models.CarModel{}, fmt.Errorf("create model image directory: %w", err)
		}
		shouldCleanupDir = true

		if coverFile != nil {
			fileName, err := saveUploadedImage(coverFile, modelDir, "cover")
			if err != nil {
				_ = tx.Rollback()
				if shouldCleanupDir {
					_ = os.RemoveAll(modelDir)
				}
				return models.CarModel{}, err
			}
			imageURL = buildUploadPath(modelID, fileName)
		}

		savedGallery := make([]string, 0, len(galleryFiles))
		for index, header := range galleryFiles {
			baseName := fmt.Sprintf("gallery_%02d", index+1)
			fileName, err := saveUploadedImage(header, modelDir, baseName)
			if err != nil {
				_ = tx.Rollback()
				if shouldCleanupDir {
					_ = os.RemoveAll(modelDir)
				}
				return models.CarModel{}, err
			}
			savedGallery = append(savedGallery, buildUploadPath(modelID, fileName))
		}

		if len(savedGallery) > 0 {
			gallery = savedGallery
		}
		if imageURL == "" && len(gallery) > 0 {
			imageURL = gallery[0]
		}

		galleryJSON, err = marshalStringSlice(gallery)
		if err != nil {
			_ = tx.Rollback()
			if shouldCleanupDir {
				_ = os.RemoveAll(modelDir)
			}
			return models.CarModel{}, fmt.Errorf("marshal uploaded gallery: %w", err)
		}

		if _, err := tx.Exec(`
			UPDATE car_models
			SET image_url = ?, gallery_json = ?, updated_at = ?
			WHERE id = ?
		`, imageURL, galleryJSON, timeText, modelID); err != nil {
			_ = tx.Rollback()
			if shouldCleanupDir {
				_ = os.RemoveAll(modelDir)
			}
			return models.CarModel{}, fmt.Errorf("update model images: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		if shouldCleanupDir {
			_ = os.RemoveAll(modelDir)
		}
		return models.CarModel{}, fmt.Errorf("commit transaction: %w", err)
	}

	return models.CarModel{
		ID:        modelID,
		Name:      req.Name,
		ModelCode: req.ModelCode,
		Brand:     req.Brand,
		Series:    req.Series,
		Scale:     req.Scale,
		Year:      req.Year,
		Color:     req.Color,
		Material:  req.Material,
		Condition: req.Condition,
		ImageURL:  imageURL,
		Gallery:   gallery,
		Notes:     req.Notes,
		Tags:      tags,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (s *ModelStore) update(id int64, req models.CreateModelRequest, coverFile *multipart.FileHeader, galleryFiles []*multipart.FileHeader) (models.CarModel, error) {
	if id <= 0 {
		return models.CarModel{}, errors.New("invalid model id")
	}

	req = normalizeCreateModelRequest(req)
	if req.Name == "" {
		return models.CarModel{}, errors.New("name is required")
	}
	if req.Year < 0 {
		return models.CarModel{}, errors.New("year must be a positive number")
	}

	existing, err := s.getByID(id)
	if err != nil {
		return models.CarModel{}, err
	}

	now := time.Now().UTC()
	timeText := now.Format(time.RFC3339Nano)
	imageURL := req.ImageURL
	if imageURL == "" {
		imageURL = existing.ImageURL
	}

	gallery := normalizeURLList(req.Gallery)
	if len(gallery) == 0 {
		gallery = existing.Gallery
	}

	tags := normalizeTags(req.Tags)
	modelDir := filepath.Join(s.imagesRoot, strconv.FormatInt(id, 10))

	if coverFile != nil || len(galleryFiles) > 0 {
		if err := os.MkdirAll(modelDir, 0o755); err != nil {
			return models.CarModel{}, fmt.Errorf("create model image directory: %w", err)
		}
	}

	if coverFile != nil {
		if err := deleteFilesByPrefix(modelDir, "cover"); err != nil {
			return models.CarModel{}, fmt.Errorf("cleanup existing cover image: %w", err)
		}

		fileName, err := saveUploadedImage(coverFile, modelDir, "cover")
		if err != nil {
			return models.CarModel{}, err
		}
		imageURL = buildUploadPath(id, fileName)
	}

	if len(galleryFiles) > 0 {
		if err := deleteFilesByPrefix(modelDir, "gallery_"); err != nil {
			return models.CarModel{}, fmt.Errorf("cleanup existing gallery images: %w", err)
		}

		savedGallery := make([]string, 0, len(galleryFiles))
		for index, header := range galleryFiles {
			baseName := fmt.Sprintf("gallery_%02d", index+1)
			fileName, err := saveUploadedImage(header, modelDir, baseName)
			if err != nil {
				return models.CarModel{}, err
			}
			savedGallery = append(savedGallery, buildUploadPath(id, fileName))
		}
		gallery = savedGallery
		if imageURL == "" && len(gallery) > 0 {
			imageURL = gallery[0]
		}
	}

	galleryJSON, err := marshalStringSlice(gallery)
	if err != nil {
		return models.CarModel{}, fmt.Errorf("marshal gallery: %w", err)
	}
	tagsJSON, err := marshalStringSlice(tags)
	if err != nil {
		return models.CarModel{}, fmt.Errorf("marshal tags: %w", err)
	}

	result, err := s.db.Exec(`
		UPDATE car_models
		SET name = ?, model_code = ?, brand = ?, series = ?, scale = ?, year = ?, color = ?, material = ?, condition = ?,
		    image_url = ?, gallery_json = ?, notes = ?, tags_json = ?, updated_at = ?
		WHERE id = ?
	`, req.Name, req.ModelCode, req.Brand, req.Series, req.Scale, req.Year, req.Color, req.Material, req.Condition,
		imageURL, galleryJSON, req.Notes, tagsJSON, timeText, id)
	if err != nil {
		return models.CarModel{}, fmt.Errorf("update model: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return models.CarModel{}, fmt.Errorf("read updated row count: %w", err)
	}
	if rows == 0 {
		return models.CarModel{}, ErrModelNotFound
	}

	return models.CarModel{
		ID:        id,
		Name:      req.Name,
		ModelCode: req.ModelCode,
		Brand:     req.Brand,
		Series:    req.Series,
		Scale:     req.Scale,
		Year:      req.Year,
		Color:     req.Color,
		Material:  req.Material,
		Condition: req.Condition,
		ImageURL:  imageURL,
		Gallery:   gallery,
		Notes:     req.Notes,
		Tags:      tags,
		CreatedAt: existing.CreatedAt,
		UpdatedAt: now,
	}, nil
}

func (s *ModelStore) getByID(id int64) (models.CarModel, error) {
	row := s.db.QueryRow(`
		SELECT id, name, model_code, brand, series, scale, year, color, material, condition,
		       image_url, gallery_json, notes, tags_json, created_at, updated_at
		FROM car_models
		WHERE id = ?
	`, id)

	item, err := scanModel(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.CarModel{}, ErrModelNotFound
	}
	if err != nil {
		return models.CarModel{}, fmt.Errorf("query model by id: %w", err)
	}

	return item, nil
}

func (s *ModelStore) initSchema() error {
	const schema = `
	CREATE TABLE IF NOT EXISTS car_models (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		model_code TEXT NOT NULL DEFAULT '',
		brand TEXT NOT NULL DEFAULT '',
		series TEXT NOT NULL DEFAULT '',
		scale TEXT NOT NULL DEFAULT '',
		year INTEGER NOT NULL DEFAULT 0,
		color TEXT NOT NULL DEFAULT '',
		material TEXT NOT NULL DEFAULT '',
		condition TEXT NOT NULL DEFAULT '',
		image_url TEXT NOT NULL DEFAULT '',
		gallery_json TEXT NOT NULL DEFAULT '[]',
		notes TEXT NOT NULL DEFAULT '',
		tags_json TEXT NOT NULL DEFAULT '[]',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("init sqlite schema: %w", err)
	}
	return nil
}

func (s *ModelStore) seedFromLegacyDataIfNeeded() error {
	if strings.TrimSpace(s.legacyDataPath) == "" {
		return nil
	}

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM car_models`).Scan(&count); err != nil {
		return fmt.Errorf("count existing models: %w", err)
	}
	if count > 0 {
		return nil
	}

	data, err := os.ReadFile(s.legacyDataPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read legacy data file: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}

	var legacyItems []models.CarModel
	if err := json.Unmarshal(data, &legacyItems); err != nil {
		log.Printf("[data] skip legacy import from %s due to parse error: %v", s.legacyDataPath, err)
		return nil
	}
	if len(legacyItems) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin legacy import transaction: %w", err)
	}

	nextID := int64(1)
	usedIDs := make(map[int64]struct{}, len(legacyItems))
	for _, item := range legacyItems {
		item = normalizeLegacyItem(item)

		if item.ID <= 0 {
			item.ID = nextID
		}
		if _, exists := usedIDs[item.ID]; exists {
			item.ID = nextID
		}
		usedIDs[item.ID] = struct{}{}
		if item.ID >= nextID {
			nextID = item.ID + 1
		}

		tagsJSON, err := marshalStringSlice(normalizeTags(item.Tags))
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("marshal legacy tags: %w", err)
		}
		galleryJSON, err := marshalStringSlice(normalizeURLList(item.Gallery))
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("marshal legacy gallery: %w", err)
		}

		if _, err := tx.Exec(`
			INSERT INTO car_models(
				id, name, model_code, brand, series, scale, year, color, material, condition,
				image_url, gallery_json, notes, tags_json, created_at, updated_at
			) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, item.ID, item.Name, item.ModelCode, item.Brand, item.Series, item.Scale, item.Year, item.Color,
			item.Material, item.Condition, item.ImageURL, galleryJSON, item.Notes, tagsJSON,
			item.CreatedAt.Format(time.RFC3339Nano), item.UpdatedAt.Format(time.RFC3339Nano)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert legacy model %d: %w", item.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit legacy import transaction: %w", err)
	}

	log.Printf("[data] imported %d models from %s into SQLite", len(legacyItems), s.legacyDataPath)
	return nil
}

func normalizeLegacyItem(item models.CarModel) models.CarModel {
	now := time.Now().UTC()
	item.Name = strings.TrimSpace(item.Name)
	item.ModelCode = strings.TrimSpace(item.ModelCode)
	item.Brand = strings.TrimSpace(item.Brand)
	item.Series = strings.TrimSpace(item.Series)
	item.Scale = strings.TrimSpace(item.Scale)
	item.Color = strings.TrimSpace(item.Color)
	item.Material = strings.TrimSpace(item.Material)
	item.Condition = strings.TrimSpace(item.Condition)
	item.ImageURL = strings.TrimSpace(item.ImageURL)
	item.Notes = strings.TrimSpace(item.Notes)
	item.Tags = normalizeTags(item.Tags)
	item.Gallery = normalizeURLList(item.Gallery)
	if item.Year < 0 {
		item.Year = 0
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = item.CreatedAt
	}
	return item
}

func scanModel(scanner interface {
	Scan(dest ...any) error
}) (models.CarModel, error) {
	var (
		item        models.CarModel
		galleryJSON string
		tagsJSON    string
		createdText string
		updatedText string
	)

	if err := scanner.Scan(
		&item.ID,
		&item.Name,
		&item.ModelCode,
		&item.Brand,
		&item.Series,
		&item.Scale,
		&item.Year,
		&item.Color,
		&item.Material,
		&item.Condition,
		&item.ImageURL,
		&galleryJSON,
		&item.Notes,
		&tagsJSON,
		&createdText,
		&updatedText,
	); err != nil {
		return models.CarModel{}, err
	}

	gallery, err := unmarshalStringSlice(galleryJSON)
	if err != nil {
		return models.CarModel{}, fmt.Errorf("unmarshal gallery: %w", err)
	}
	tags, err := unmarshalStringSlice(tagsJSON)
	if err != nil {
		return models.CarModel{}, fmt.Errorf("unmarshal tags: %w", err)
	}

	item.Gallery = gallery
	item.Tags = tags
	item.CreatedAt = parseTimeOrZero(createdText)
	item.UpdatedAt = parseTimeOrZero(updatedText)
	return item, nil
}

func parseTimeOrZero(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func normalizeCreateModelRequest(req models.CreateModelRequest) models.CreateModelRequest {
	req.Name = strings.TrimSpace(req.Name)
	req.ModelCode = strings.TrimSpace(req.ModelCode)
	req.Brand = strings.TrimSpace(req.Brand)
	req.Series = strings.TrimSpace(req.Series)
	req.Scale = strings.TrimSpace(req.Scale)
	req.Color = strings.TrimSpace(req.Color)
	req.Material = strings.TrimSpace(req.Material)
	req.Condition = strings.TrimSpace(req.Condition)
	req.ImageURL = strings.TrimSpace(req.ImageURL)
	req.Notes = strings.TrimSpace(req.Notes)
	req.Tags = normalizeTags(req.Tags)
	req.Gallery = normalizeURLList(req.Gallery)
	if req.Year < 0 {
		req.Year = 0
	}
	return req
}

func normalizeTags(tags []string) []string {
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		value := strings.TrimSpace(tag)
		if value == "" {
			continue
		}
		normalized = append(normalized, value)
	}
	return normalized
}

func normalizeURLList(urls []string) []string {
	normalized := make([]string, 0, len(urls))
	seen := make(map[string]struct{}, len(urls))

	for _, raw := range urls {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}

	return normalized
}

func marshalStringSlice(values []string) (string, error) {
	payload, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func unmarshalStringSlice(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}, nil
	}

	var values []string
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil, err
	}
	return normalizeURLList(values), nil
}

func saveUploadedImage(fileHeader *multipart.FileHeader, modelDir, baseName string) (string, error) {
	if fileHeader == nil {
		return "", errors.New("empty file header")
	}
	if fileHeader.Size > maxUploadBytes {
		return "", fmt.Errorf("%s exceeds max upload size", fileHeader.Filename)
	}

	source, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("open uploaded file %s: %w", fileHeader.Filename, err)
	}
	defer source.Close()

	sniffBuf := make([]byte, 512)
	readSize, err := source.Read(sniffBuf)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read uploaded file %s: %w", fileHeader.Filename, err)
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("reset uploaded file %s: %w", fileHeader.Filename, err)
	}

	contentType := http.DetectContentType(sniffBuf[:readSize])
	if !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("%s is not a supported image", fileHeader.Filename)
	}

	ext := normalizeImageExtension(fileHeader.Filename, contentType)
	if ext == "" {
		return "", fmt.Errorf("unsupported image format for %s", fileHeader.Filename)
	}

	fileName := baseName + ext
	destinationPath := filepath.Join(modelDir, fileName)
	destination, err := os.Create(destinationPath)
	if err != nil {
		return "", fmt.Errorf("create destination image %s: %w", destinationPath, err)
	}
	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		return "", fmt.Errorf("save image %s: %w", destinationPath, err)
	}

	return fileName, nil
}

func normalizeImageExtension(filename, contentType string) string {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	if isAllowedImageExtension(ext) {
		return ext
	}

	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	case "image/avif":
		return ".avif"
	default:
		return ""
	}
}

func isAllowedImageExtension(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".avif":
		return true
	default:
		return false
	}
}

func buildUploadPath(modelID int64, fileName string) string {
	return path.Join("/uploads", strconv.FormatInt(modelID, 10), fileName)
}

func deleteFilesByPrefix(dir, prefix string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	lowerPrefix := strings.ToLower(strings.TrimSpace(prefix))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(strings.ToLower(name), lowerPrefix) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	return nil
}
