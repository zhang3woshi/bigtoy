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

	"github.com/google/uuid"
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

func (s *ModelStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
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

func (s *ModelStore) ReplaceAll(rawItems []models.CarModel) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin import transaction: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM car_models`); err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("clear existing models: %w", err)
	}

	usedIDs := make(map[string]struct{}, len(rawItems))
	for index, rawItem := range rawItems {
		item := normalizeLegacyItem(rawItem)
		if item.Name == "" {
			_ = tx.Rollback()
			return 0, fmt.Errorf("model #%d name is required", index+1)
		}
		item.ID = allocateLegacyID(item.ID, usedIDs)

		if err := insertLegacyModelTx(tx, item); err != nil {
			_ = tx.Rollback()
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit import transaction: %w", err)
	}

	return len(rawItems), nil
}

func (s *ModelStore) Add(req models.CreateModelRequest) (models.CarModel, error) {
	return s.add(req, nil, nil)
}

func (s *ModelStore) AddWithUploads(req models.CreateModelRequest, coverFile *multipart.FileHeader, galleryFiles []*multipart.FileHeader) (models.CarModel, error) {
	return s.add(req, coverFile, galleryFiles)
}

func (s *ModelStore) Update(id string, req models.CreateModelRequest) (models.CarModel, error) {
	return s.update(id, req, nil, nil)
}

func (s *ModelStore) UpdateWithUploads(id string, req models.CreateModelRequest, coverFile *multipart.FileHeader, galleryFiles []*multipart.FileHeader) (models.CarModel, error) {
	return s.update(id, req, coverFile, galleryFiles)
}

func (s *ModelStore) Delete(id string) error {
	normalizedID, err := normalizeModelID(id)
	if err != nil {
		return errors.New("invalid model id")
	}

	existing, err := s.getByID(normalizedID)
	if err != nil {
		if errors.Is(err, ErrModelNotFound) {
			return ErrModelNotFound
		}
		return err
	}

	result, err := s.db.Exec(`DELETE FROM car_models WHERE id = ?`, normalizedID)
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

	for _, dir := range collectImageDirsForModel(s.imagesRoot, existing, normalizedID) {
		if err := os.RemoveAll(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("remove model image directory failed (id=%s): %v", normalizedID, err)
		}
	}

	return nil
}

type modelWriteState struct {
	now      time.Time
	timeText string
	imageURL string
	gallery  []string
	tags     []string
}

func (s *ModelStore) add(req models.CreateModelRequest, coverFile *multipart.FileHeader, galleryFiles []*multipart.FileHeader) (models.CarModel, error) {
	preparedReq, state, err := prepareModelWriteState(req)
	if err != nil {
		return models.CarModel{}, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return models.CarModel{}, fmt.Errorf("begin transaction: %w", err)
	}

	modelID, err := s.insertModelTx(tx, preparedReq, state)
	if err != nil {
		_ = tx.Rollback()
		return models.CarModel{}, err
	}

	cleanupDir, err := s.applyCreateUploads(tx, modelID, coverFile, galleryFiles, &state)
	if err != nil {
		_ = tx.Rollback()
		s.cleanupCreatedModelDir(modelID, cleanupDir)
		return models.CarModel{}, err
	}

	if err := tx.Commit(); err != nil {
		s.cleanupCreatedModelDir(modelID, cleanupDir)
		return models.CarModel{}, fmt.Errorf("commit transaction: %w", err)
	}

	return buildModelResponse(modelID, preparedReq, state, state.now), nil
}

func prepareModelWriteState(req models.CreateModelRequest) (models.CreateModelRequest, modelWriteState, error) {
	req = normalizeCreateModelRequest(req)
	if err := validateModelRequest(req); err != nil {
		return models.CreateModelRequest{}, modelWriteState{}, err
	}

	now := time.Now().UTC()
	state := modelWriteState{
		now:      now,
		timeText: now.Format(time.RFC3339Nano),
		imageURL: req.ImageURL,
		gallery:  normalizeURLList(req.Gallery),
		tags:     normalizeTags(req.Tags),
	}
	return req, state, nil
}

func validateModelRequest(req models.CreateModelRequest) error {
	if req.Name == "" {
		return errors.New("name is required")
	}
	if req.Year < 0 {
		return errors.New("year must be a positive number")
	}
	return nil
}

func (s *ModelStore) insertModelTx(tx *sql.Tx, req models.CreateModelRequest, state modelWriteState) (string, error) {
	modelID := uuid.NewString()

	galleryJSON, err := marshalStringSlice(state.gallery)
	if err != nil {
		return "", fmt.Errorf("marshal gallery: %w", err)
	}
	tagsJSON, err := marshalStringSlice(state.tags)
	if err != nil {
		return "", fmt.Errorf("marshal tags: %w", err)
	}

	if _, err := tx.Exec(`
		INSERT INTO car_models(
			id,
			name, model_code, brand, series, scale, year, color, material, condition,
			image_url, gallery_json, notes, tags_json, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, modelID, req.Name, req.ModelCode, req.Brand, req.Series, req.Scale, req.Year, req.Color, req.Material, req.Condition,
		state.imageURL, galleryJSON, req.Notes, tagsJSON, state.timeText, state.timeText); err != nil {
		return "", fmt.Errorf("insert model: %w", err)
	}
	return modelID, nil
}

func (s *ModelStore) applyCreateUploads(tx *sql.Tx, modelID string, coverFile *multipart.FileHeader, galleryFiles []*multipart.FileHeader, state *modelWriteState) (bool, error) {
	if coverFile == nil && len(galleryFiles) == 0 {
		return false, nil
	}

	modelDir := modelImageDir(s.imagesRoot, modelID)
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		return false, fmt.Errorf("create model image directory: %w", err)
	}

	if err := applyCoverUpload(modelID, modelDir, coverFile, &state.imageURL); err != nil {
		return true, err
	}
	if err := applyGalleryUpload(modelID, modelDir, galleryFiles, &state.gallery); err != nil {
		return true, err
	}
	ensureImageURLFromGallery(&state.imageURL, state.gallery)

	if err := updateModelImagesTx(tx, modelID, state.imageURL, state.gallery, state.timeText); err != nil {
		return true, err
	}
	return true, nil
}

func applyCoverUpload(modelID string, modelDir string, coverFile *multipart.FileHeader, imageURL *string) error {
	if coverFile == nil {
		return nil
	}

	fileName, err := saveUploadedImage(coverFile, modelDir, "cover")
	if err != nil {
		return err
	}
	*imageURL = buildUploadPath(modelID, fileName)
	return nil
}

func applyGalleryUpload(modelID string, modelDir string, galleryFiles []*multipart.FileHeader, gallery *[]string) error {
	if len(galleryFiles) == 0 {
		return nil
	}

	savedGallery, err := saveGalleryUploads(modelID, modelDir, galleryFiles)
	if err != nil {
		return err
	}
	*gallery = savedGallery
	return nil
}

func saveGalleryUploads(modelID string, modelDir string, galleryFiles []*multipart.FileHeader) ([]string, error) {
	savedGallery := make([]string, 0, len(galleryFiles))
	for index, header := range galleryFiles {
		baseName := fmt.Sprintf("gallery_%02d", index+1)
		fileName, err := saveUploadedImage(header, modelDir, baseName)
		if err != nil {
			return nil, err
		}
		savedGallery = append(savedGallery, buildUploadPath(modelID, fileName))
	}
	return savedGallery, nil
}

func ensureImageURLFromGallery(imageURL *string, gallery []string) {
	if *imageURL == "" && len(gallery) > 0 {
		*imageURL = gallery[0]
	}
}

func updateModelImagesTx(tx *sql.Tx, modelID string, imageURL string, gallery []string, updatedAt string) error {
	galleryJSON, err := marshalStringSlice(gallery)
	if err != nil {
		return fmt.Errorf("marshal uploaded gallery: %w", err)
	}

	if _, err := tx.Exec(`
		UPDATE car_models
		SET image_url = ?, gallery_json = ?, updated_at = ?
		WHERE id = ?
	`, imageURL, galleryJSON, updatedAt, modelID); err != nil {
		return fmt.Errorf("update model images: %w", err)
	}
	return nil
}

func buildModelResponse(id string, req models.CreateModelRequest, state modelWriteState, createdAt time.Time) models.CarModel {
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
		ImageURL:  state.imageURL,
		Gallery:   state.gallery,
		Notes:     req.Notes,
		Tags:      state.tags,
		CreatedAt: createdAt,
		UpdatedAt: state.now,
	}
}

func modelImageDir(imagesRoot string, modelID string) string {
	return filepath.Join(imagesRoot, modelID)
}

func (s *ModelStore) cleanupCreatedModelDir(modelID string, shouldCleanup bool) {
	if !shouldCleanup {
		return
	}
	_ = os.RemoveAll(modelImageDir(s.imagesRoot, modelID))
}

func (s *ModelStore) update(id string, req models.CreateModelRequest, coverFile *multipart.FileHeader, galleryFiles []*multipart.FileHeader) (models.CarModel, error) {
	normalizedID, err := normalizeModelID(id)
	if err != nil {
		return models.CarModel{}, errors.New("invalid model id")
	}

	preparedReq, state, createdAt, err := s.prepareModelUpdateState(normalizedID, req)
	if err != nil {
		return models.CarModel{}, err
	}

	if err := s.applyUpdateUploads(normalizedID, coverFile, galleryFiles, &state); err != nil {
		return models.CarModel{}, err
	}

	if err := s.persistModelUpdate(normalizedID, preparedReq, state); err != nil {
		return models.CarModel{}, err
	}

	return buildModelResponse(normalizedID, preparedReq, state, createdAt), nil
}

func (s *ModelStore) prepareModelUpdateState(id string, req models.CreateModelRequest) (models.CreateModelRequest, modelWriteState, time.Time, error) {
	id, err := normalizeModelID(id)
	if err != nil {
		return models.CreateModelRequest{}, modelWriteState{}, time.Time{}, errors.New("invalid model id")
	}

	req = normalizeCreateModelRequest(req)
	if err := validateModelRequest(req); err != nil {
		return models.CreateModelRequest{}, modelWriteState{}, time.Time{}, err
	}

	existing, err := s.getByID(id)
	if err != nil {
		return models.CreateModelRequest{}, modelWriteState{}, time.Time{}, err
	}

	now := time.Now().UTC()
	state := modelWriteState{
		now:      now,
		timeText: now.Format(time.RFC3339Nano),
		imageURL: req.ImageURL,
		gallery:  normalizeURLList(req.Gallery),
		tags:     normalizeTags(req.Tags),
	}
	if state.imageURL == "" {
		state.imageURL = existing.ImageURL
	}
	if len(state.gallery) == 0 {
		state.gallery = existing.Gallery
	}

	return req, state, existing.CreatedAt, nil
}

func (s *ModelStore) applyUpdateUploads(id string, coverFile *multipart.FileHeader, galleryFiles []*multipart.FileHeader, state *modelWriteState) error {
	if coverFile == nil && len(galleryFiles) == 0 {
		return nil
	}

	modelDir := modelImageDir(s.imagesRoot, id)
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		return fmt.Errorf("create model image directory: %w", err)
	}

	if err := replaceCoverUpload(id, modelDir, coverFile, &state.imageURL); err != nil {
		return err
	}
	if err := replaceGalleryUpload(id, modelDir, galleryFiles, &state.gallery, &state.imageURL); err != nil {
		return err
	}
	return nil
}

func replaceCoverUpload(modelID string, modelDir string, coverFile *multipart.FileHeader, imageURL *string) error {
	if coverFile == nil {
		return nil
	}

	if err := deleteFilesByPrefix(modelDir, "cover"); err != nil {
		return fmt.Errorf("cleanup existing cover image: %w", err)
	}
	if err := applyCoverUpload(modelID, modelDir, coverFile, imageURL); err != nil {
		return err
	}
	return nil
}

func replaceGalleryUpload(modelID string, modelDir string, galleryFiles []*multipart.FileHeader, gallery *[]string, imageURL *string) error {
	if len(galleryFiles) == 0 {
		return nil
	}

	if err := deleteFilesByPrefix(modelDir, "gallery_"); err != nil {
		return fmt.Errorf("cleanup existing gallery images: %w", err)
	}
	if err := applyGalleryUpload(modelID, modelDir, galleryFiles, gallery); err != nil {
		return err
	}
	ensureImageURLFromGallery(imageURL, *gallery)
	return nil
}

func (s *ModelStore) persistModelUpdate(id string, req models.CreateModelRequest, state modelWriteState) error {
	galleryJSON, err := marshalStringSlice(state.gallery)
	if err != nil {
		return fmt.Errorf("marshal gallery: %w", err)
	}
	tagsJSON, err := marshalStringSlice(state.tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	result, err := s.db.Exec(`
		UPDATE car_models
		SET name = ?, model_code = ?, brand = ?, series = ?, scale = ?, year = ?, color = ?, material = ?, condition = ?,
		    image_url = ?, gallery_json = ?, notes = ?, tags_json = ?, updated_at = ?
		WHERE id = ?
	`, req.Name, req.ModelCode, req.Brand, req.Series, req.Scale, req.Year, req.Color, req.Material, req.Condition,
		state.imageURL, galleryJSON, req.Notes, tagsJSON, state.timeText, id)
	if err != nil {
		return fmt.Errorf("update model: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated row count: %w", err)
	}
	if rows == 0 {
		return ErrModelNotFound
	}
	return nil
}

func (s *ModelStore) getByID(id string) (models.CarModel, error) {
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
	if _, err := s.db.Exec(createCarModelsTableSQL("car_models")); err != nil {
		return fmt.Errorf("init sqlite schema: %w", err)
	}

	idColumnType, err := s.getModelIDColumnType()
	if err != nil {
		return err
	}
	if strings.Contains(strings.ToUpper(idColumnType), "INT") {
		if err := s.migrateNumericIDsToUUID(); err != nil {
			return err
		}
	}

	return nil
}

func (s *ModelStore) seedFromLegacyDataIfNeeded() error {
	if !s.hasLegacySeedPath() {
		return nil
	}

	shouldSeed, err := s.isLegacySeedRequired()
	if err != nil {
		return err
	}
	if !shouldSeed {
		return nil
	}

	data, shouldImport, err := s.loadLegacySeedData()
	if err != nil {
		return err
	}
	if !shouldImport {
		return nil
	}

	legacyItems, shouldImport := parseLegacySeedModels(data, s.legacyDataPath)
	if !shouldImport {
		return nil
	}

	if err := s.importLegacyModels(legacyItems); err != nil {
		return err
	}

	log.Printf("[data] imported %d models from %s into SQLite", len(legacyItems), s.legacyDataPath)
	return nil
}

func (s *ModelStore) hasLegacySeedPath() bool {
	return strings.TrimSpace(s.legacyDataPath) != ""
}

func (s *ModelStore) isLegacySeedRequired() (bool, error) {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM car_models`).Scan(&count); err != nil {
		return false, fmt.Errorf("count existing models: %w", err)
	}
	return count == 0, nil
}

func (s *ModelStore) loadLegacySeedData() ([]byte, bool, error) {
	data, err := os.ReadFile(s.legacyDataPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read legacy data file: %w", err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil, false, nil
	}
	return data, true, nil
}

func parseLegacySeedModels(data []byte, sourcePath string) ([]models.CarModel, bool) {
	var rawItems []map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawItems); err != nil {
		log.Printf("[data] skip legacy import from %s due to parse error: %v", sourcePath, err)
		return nil, false
	}
	if len(rawItems) == 0 {
		return nil, false
	}

	legacyItems := make([]models.CarModel, 0, len(rawItems))
	for _, rawItem := range rawItems {
		idJSON, err := json.Marshal(parseLegacyRawID(rawItem["id"]))
		if err != nil {
			log.Printf("[data] skip legacy import from %s due to id conversion error: %v", sourcePath, err)
			return nil, false
		}
		rawItem["id"] = idJSON

		itemJSON, err := json.Marshal(rawItem)
		if err != nil {
			log.Printf("[data] skip legacy import from %s due to conversion error: %v", sourcePath, err)
			return nil, false
		}

		var item models.CarModel
		if err := json.Unmarshal(itemJSON, &item); err != nil {
			log.Printf("[data] skip legacy import from %s due to parse error: %v", sourcePath, err)
			return nil, false
		}
		legacyItems = append(legacyItems, item)
	}

	return legacyItems, true
}

func (s *ModelStore) importLegacyModels(legacyItems []models.CarModel) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin legacy import transaction: %w", err)
	}

	usedIDs := make(map[string]struct{}, len(legacyItems))
	for _, raw := range legacyItems {
		item := normalizeLegacyItem(raw)
		item.ID = allocateLegacyID(item.ID, usedIDs)

		if err := insertLegacyModelTx(tx, item); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit legacy import transaction: %w", err)
	}
	return nil
}

func insertLegacyModelTx(tx *sql.Tx, item models.CarModel) error {
	tagsJSON, err := marshalStringSlice(normalizeTags(item.Tags))
	if err != nil {
		return fmt.Errorf("marshal legacy tags: %w", err)
	}
	galleryJSON, err := marshalStringSlice(normalizeURLList(item.Gallery))
	if err != nil {
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
		return fmt.Errorf("insert legacy model %s: %w", item.ID, err)
	}
	return nil
}

func normalizeLegacyItem(item models.CarModel) models.CarModel {
	now := time.Now().UTC()
	item.ID = strings.TrimSpace(item.ID)
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

func buildUploadPath(modelID string, fileName string) string {
	return path.Join("/uploads", modelID, fileName)
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

func createCarModelsTableSQL(tableName string) string {
	return fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s (
		id TEXT PRIMARY KEY,
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
	);`, tableName)
}

func (s *ModelStore) getModelIDColumnType() (string, error) {
	rows, err := s.db.Query(`PRAGMA table_info(car_models)`)
	if err != nil {
		return "", fmt.Errorf("inspect sqlite schema: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid          int
			name         string
			columnType   string
			notNull      int
			defaultValue sql.NullString
			primaryKey   int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return "", fmt.Errorf("read sqlite schema: %w", err)
		}
		if strings.EqualFold(strings.TrimSpace(name), "id") {
			return strings.TrimSpace(columnType), nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate sqlite schema: %w", err)
	}
	return "", fmt.Errorf("car_models.id column is missing")
}

func (s *ModelStore) migrateNumericIDsToUUID() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin id migration transaction: %w", err)
	}

	const migrationTable = "car_models_uuid_migration"
	if _, err := tx.Exec(`DROP TABLE IF EXISTS ` + migrationTable); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("drop migration table: %w", err)
	}
	if _, err := tx.Exec(createCarModelsTableSQL(migrationTable)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("create migration table: %w", err)
	}

	rows, err := tx.Query(`
		SELECT id, name, model_code, brand, series, scale, year, color, material, condition,
		       image_url, gallery_json, notes, tags_json, created_at, updated_at
		FROM car_models
	`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("query legacy models for id migration: %w", err)
	}

	insertStmt, err := tx.Prepare(`
		INSERT INTO car_models_uuid_migration(
			id, name, model_code, brand, series, scale, year, color, material, condition,
			image_url, gallery_json, notes, tags_json, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		rows.Close()
		_ = tx.Rollback()
		return fmt.Errorf("prepare id migration insert: %w", err)
	}

	migratedCount := 0
	for rows.Next() {
		var (
			legacyID    int64
			name        string
			modelCode   string
			brand       string
			series      string
			scale       string
			year        int
			color       string
			material    string
			condition   string
			imageURL    string
			galleryJSON string
			notes       string
			tagsJSON    string
			createdAt   string
			updatedAt   string
		)
		if err := rows.Scan(
			&legacyID,
			&name,
			&modelCode,
			&brand,
			&series,
			&scale,
			&year,
			&color,
			&material,
			&condition,
			&imageURL,
			&galleryJSON,
			&notes,
			&tagsJSON,
			&createdAt,
			&updatedAt,
		); err != nil {
			insertStmt.Close()
			rows.Close()
			_ = tx.Rollback()
			return fmt.Errorf("scan id migration row: %w", err)
		}

		newID := uuid.NewString()
		if _, err := insertStmt.Exec(
			newID,
			name,
			modelCode,
			brand,
			series,
			scale,
			year,
			color,
			material,
			condition,
			imageURL,
			galleryJSON,
			notes,
			tagsJSON,
			createdAt,
			updatedAt,
		); err != nil {
			insertStmt.Close()
			rows.Close()
			_ = tx.Rollback()
			return fmt.Errorf("insert migrated model (legacy id=%d): %w", legacyID, err)
		}
		migratedCount++
	}
	if err := rows.Err(); err != nil {
		insertStmt.Close()
		rows.Close()
		_ = tx.Rollback()
		return fmt.Errorf("iterate id migration rows: %w", err)
	}

	if err := insertStmt.Close(); err != nil {
		rows.Close()
		_ = tx.Rollback()
		return fmt.Errorf("close id migration statement: %w", err)
	}
	if err := rows.Close(); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("close id migration rows: %w", err)
	}

	if _, err := tx.Exec(`DROP TABLE car_models`); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("drop legacy car_models table: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE ` + migrationTable + ` RENAME TO car_models`); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("rename migrated car_models table: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit id migration transaction: %w", err)
	}

	log.Printf("[data] migrated car_models ids from integer to UUID (%d rows)", migratedCount)
	return nil
}

func parseLegacyRawID(rawID json.RawMessage) string {
	trimmed := strings.TrimSpace(string(rawID))
	if trimmed == "" || trimmed == "null" {
		return ""
	}

	var parsedString string
	if err := json.Unmarshal(rawID, &parsedString); err == nil {
		return strings.TrimSpace(parsedString)
	}

	var parsedInt int64
	if err := json.Unmarshal(rawID, &parsedInt); err == nil && parsedInt > 0 {
		return strconv.FormatInt(parsedInt, 10)
	}

	return ""
}

func allocateLegacyID(candidate string, usedIDs map[string]struct{}) string {
	id := strings.TrimSpace(candidate)
	if _, err := uuid.Parse(id); err != nil {
		id = ""
	}
	if id == "" {
		id = uuid.NewString()
	}

	for {
		if _, exists := usedIDs[id]; !exists {
			usedIDs[id] = struct{}{}
			return id
		}
		id = uuid.NewString()
	}
}

func normalizeModelID(rawID string) (string, error) {
	id := strings.TrimSpace(rawID)
	if id == "" {
		return "", errors.New("model id is required")
	}
	if _, err := uuid.Parse(id); err != nil {
		return "", errors.New("invalid model id")
	}
	return id, nil
}

func collectImageDirsForModel(imagesRoot string, item models.CarModel, fallbackModelID string) []string {
	uniqueDirs := make(map[string]struct{})
	addDir := func(modelID string) {
		trimmedID := strings.TrimSpace(modelID)
		if trimmedID == "" {
			return
		}
		uniqueDirs[modelImageDir(imagesRoot, trimmedID)] = struct{}{}
	}

	addDir(item.ID)
	addDir(fallbackModelID)
	addDir(modelIDFromUploadPath(item.ImageURL))
	for _, imageURL := range item.Gallery {
		addDir(modelIDFromUploadPath(imageURL))
	}

	dirs := make([]string, 0, len(uniqueDirs))
	for dir := range uniqueDirs {
		dirs = append(dirs, dir)
	}
	return dirs
}

func modelIDFromUploadPath(rawURL string) string {
	parsedPath := path.Clean(strings.TrimSpace(rawURL))
	if !strings.HasPrefix(parsedPath, "/uploads/") {
		return ""
	}

	parts := strings.Split(strings.TrimPrefix(parsedPath, "/uploads/"), "/")
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}
