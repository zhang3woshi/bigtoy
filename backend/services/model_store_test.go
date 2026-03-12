package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"bigtoy/backend/models"
)

func newModelStoreForTest(t *testing.T, legacyPath string) *ModelStore {
	t.Helper()

	root := t.TempDir()
	dbPath := filepath.Join(root, "data", "models.db")
	imagesRoot := filepath.Join(root, "images")

	store, err := NewModelStore(dbPath, imagesRoot, legacyPath)
	if err != nil {
		t.Fatalf("new model store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func makeUploadHeader(t *testing.T, field, fileName, contentType string, content []byte) *multipart.FileHeader {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, field, fileName))
	partHeader.Set("Content-Type", contentType)

	part, err := writer.CreatePart(partHeader)
	if err != nil {
		t.Fatalf("create multipart part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(maxUploadBytes); err != nil {
		t.Fatalf("parse multipart form: %v", err)
	}

	files := request.MultipartForm.File[field]
	if len(files) == 0 {
		t.Fatalf("missing multipart file header for field %s", field)
	}
	t.Cleanup(func() {
		if request.MultipartForm != nil {
			_ = request.MultipartForm.RemoveAll()
		}
	})
	return files[0]
}

func TestModelStoreCRUD(t *testing.T) {
	store := newModelStoreForTest(t, "")

	if _, err := store.Add(models.CreateModelRequest{}); err == nil {
		t.Fatal("expected validation error for empty model name")
	}

	created, err := store.Add(models.CreateModelRequest{
		Name:      "  Skyline GT-R  ",
		ModelCode: " BNR34 ",
		Brand:     " Nissan ",
		Year:      1999,
		Gallery:   []string{" /uploads/1/a.png ", "/uploads/1/a.png", "/uploads/1/b.png"},
		Tags:      []string{" JDM ", "", "JDM", " R34 "},
	})
	if err != nil {
		t.Fatalf("add model: %v", err)
	}
	if created.ID <= 0 {
		t.Fatalf("expected positive id, got %d", created.ID)
	}
	if created.Name != "Skyline GT-R" {
		t.Fatalf("name should be normalized, got %q", created.Name)
	}
	if !slices.Equal(created.Tags, []string{"JDM", "JDM", "R34"}) {
		t.Fatalf("unexpected tags: %#v", created.Tags)
	}
	if !slices.Equal(created.Gallery, []string{"/uploads/1/a.png", "/uploads/1/b.png"}) {
		t.Fatalf("unexpected gallery: %#v", created.Gallery)
	}

	listed := store.List()
	if len(listed) != 1 {
		t.Fatalf("expected one model, got %d", len(listed))
	}

	updated, err := store.Update(created.ID, models.CreateModelRequest{
		Name:      "  Supra  ",
		ModelCode: " A80 ",
		Brand:     " Toyota ",
		Year:      2002,
	})
	if err != nil {
		t.Fatalf("update model: %v", err)
	}
	if updated.Name != "Supra" || updated.ModelCode != "A80" || updated.Brand != "Toyota" {
		t.Fatalf("unexpected updated model: %#v", updated)
	}

	if err := store.Delete(created.ID); err != nil {
		t.Fatalf("delete model: %v", err)
	}
	if err := store.Delete(created.ID); err == nil || !errorsIs(err, ErrModelNotFound) {
		t.Fatalf("expected ErrModelNotFound, got: %v", err)
	}
}

func TestModelStoreLegacySeedImport(t *testing.T) {
	root := t.TempDir()
	legacyPath := filepath.Join(root, "legacy.json")

	payload := []models.CarModel{
		{
			ID:      1,
			Name:    "  Legacy One ",
			Year:    -1,
			Tags:    []string{" old ", ""},
			Gallery: []string{" /uploads/1/a.png ", "/uploads/1/a.png"},
		},
		{
			ID:   1,
			Name: "Legacy Two",
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal legacy payload: %v", err)
	}
	if err := os.WriteFile(legacyPath, data, 0o644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	store := newModelStoreForTest(t, legacyPath)
	items := store.List()
	if len(items) != 2 {
		t.Fatalf("expected 2 imported models, got %d", len(items))
	}

	if items[0].ID == items[1].ID {
		t.Fatalf("expected unique IDs after import, got duplicates: %d", items[0].ID)
	}
	if items[0].Year != 0 {
		t.Fatalf("expected normalized year 0 for negative legacy year, got %d", items[0].Year)
	}
	if items[0].CreatedAt.IsZero() || items[0].UpdatedAt.IsZero() {
		t.Fatal("expected legacy timestamps to be normalized")
	}
}

func TestPrepareModelWriteStateValidation(t *testing.T) {
	_, _, err := prepareModelWriteState(models.CreateModelRequest{Name: "   "})
	if err == nil {
		t.Fatal("expected validation error for blank name")
	}

	req, state, err := prepareModelWriteState(models.CreateModelRequest{
		Name:     "  BMW M3 ",
		Year:     -3,
		ImageURL: " /uploads/10/cover.png ",
		Gallery:  []string{" /uploads/10/1.png ", "/uploads/10/1.png", "/uploads/10/2.png"},
		Tags:     []string{" track ", "", "fast"},
	})
	if err != nil {
		t.Fatalf("prepare model write state: %v", err)
	}

	if req.Name != "BMW M3" {
		t.Fatalf("expected normalized name, got %q", req.Name)
	}
	if req.Year != 0 {
		t.Fatalf("expected normalized year 0, got %d", req.Year)
	}
	if state.imageURL != "/uploads/10/cover.png" {
		t.Fatalf("unexpected image url: %q", state.imageURL)
	}
	if !slices.Equal(state.gallery, []string{"/uploads/10/1.png", "/uploads/10/2.png"}) {
		t.Fatalf("unexpected normalized gallery: %#v", state.gallery)
	}
	if !slices.Equal(state.tags, []string{"track", "fast"}) {
		t.Fatalf("unexpected normalized tags: %#v", state.tags)
	}
}

func TestModelStoreUtilityFunctions(t *testing.T) {
	if parsed := parseTimeOrZero("not-a-time"); !parsed.IsZero() {
		t.Fatalf("expected zero time for invalid input, got %v", parsed)
	}
	if path := buildUploadPath(42, "cover.png"); path != "/uploads/42/cover.png" {
		t.Fatalf("unexpected upload path: %s", path)
	}

	if normalizeImageExtension("cover.PNG", "") != ".png" {
		t.Fatal("expected extension from filename")
	}
	if normalizeImageExtension("cover.unknown", "image/jpeg") != ".jpg" {
		t.Fatal("expected extension fallback from content-type")
	}
	if normalizeImageExtension("cover.unknown", "application/json") != "" {
		t.Fatal("expected unsupported content-type to return empty extension")
	}
	if !isAllowedImageExtension(".webp") {
		t.Fatal("expected .webp to be allowed")
	}
	if isAllowedImageExtension(".txt") {
		t.Fatal("expected .txt to be rejected")
	}

	jsonText, err := marshalStringSlice([]string{"a", "b"})
	if err != nil {
		t.Fatalf("marshal string slice: %v", err)
	}
	decoded, err := unmarshalStringSlice(jsonText)
	if err != nil {
		t.Fatalf("unmarshal string slice: %v", err)
	}
	if !slices.Equal(decoded, []string{"a", "b"}) {
		t.Fatalf("unexpected decoded strings: %#v", decoded)
	}
	if _, err := unmarshalStringSlice("{not-json"); err == nil {
		t.Fatal("expected invalid json error")
	}
}

func TestSaveUploadedImageAndUploadReplacement(t *testing.T) {
	modelDir := t.TempDir()
	pngBytes := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	}

	coverHeader := makeUploadHeader(t, "imageFile", "cover.png", "image/png", pngBytes)
	fileName, err := saveUploadedImage(coverHeader, modelDir, "cover")
	if err != nil {
		t.Fatalf("save uploaded image: %v", err)
	}
	if fileName != "cover.png" {
		t.Fatalf("unexpected saved file name: %s", fileName)
	}

	textHeader := makeUploadHeader(t, "imageFile", "note.txt", "text/plain", []byte("hello"))
	if _, err := saveUploadedImage(textHeader, modelDir, "cover"); err == nil {
		t.Fatal("expected non-image upload error")
	}

	imageURL := ""
	if err := applyCoverUpload(7, modelDir, coverHeader, &imageURL); err != nil {
		t.Fatalf("apply cover upload: %v", err)
	}
	if !strings.HasPrefix(imageURL, "/uploads/7/cover") {
		t.Fatalf("unexpected cover image url: %s", imageURL)
	}

	galleryHeaders := []*multipart.FileHeader{
		makeUploadHeader(t, "galleryFiles", "a.png", "image/png", pngBytes),
		makeUploadHeader(t, "galleryFiles", "b.png", "image/png", pngBytes),
	}
	gallery, err := saveGalleryUploads(7, modelDir, galleryHeaders)
	if err != nil {
		t.Fatalf("save gallery uploads: %v", err)
	}
	if len(gallery) != 2 {
		t.Fatalf("expected 2 gallery images, got %d", len(gallery))
	}

	updatedGallery := []string{}
	if err := replaceGalleryUpload(7, modelDir, galleryHeaders[:1], &updatedGallery, &imageURL); err != nil {
		t.Fatalf("replace gallery upload: %v", err)
	}
	if len(updatedGallery) != 1 {
		t.Fatalf("expected replaced gallery size 1, got %d", len(updatedGallery))
	}

	if err := replaceCoverUpload(7, modelDir, coverHeader, &imageURL); err != nil {
		t.Fatalf("replace cover upload: %v", err)
	}
}

func TestModelStoreAddAndUpdateWithUploads(t *testing.T) {
	store := newModelStoreForTest(t, "")
	pngBytes := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	}

	cover := makeUploadHeader(t, "imageFile", "cover.png", "image/png", pngBytes)
	gallery := []*multipart.FileHeader{
		makeUploadHeader(t, "galleryFiles", "gallery01.png", "image/png", pngBytes),
		makeUploadHeader(t, "galleryFiles", "gallery02.png", "image/png", pngBytes),
	}

	created, err := store.AddWithUploads(models.CreateModelRequest{
		Name:  "Upload Model",
		Brand: "BigToy",
		Year:  2020,
	}, cover, gallery)
	if err != nil {
		t.Fatalf("add with uploads: %v", err)
	}
	if created.ID <= 0 || created.ImageURL == "" || len(created.Gallery) != 2 {
		t.Fatalf("unexpected created model with uploads: %#v", created)
	}

	modelDir := modelImageDir(store.imagesRoot, created.ID)
	if _, err := os.Stat(filepath.Join(modelDir, "cover.png")); err != nil {
		t.Fatalf("expected saved cover file: %v", err)
	}

	updated, err := store.UpdateWithUploads(created.ID, models.CreateModelRequest{
		Name:  "Updated Upload Model",
		Brand: "BigToy",
		Year:  2021,
	}, makeUploadHeader(t, "imageFile", "cover-new.png", "image/png", pngBytes), []*multipart.FileHeader{
		makeUploadHeader(t, "galleryFiles", "gallery-new.png", "image/png", pngBytes),
	})
	if err != nil {
		t.Fatalf("update with uploads: %v", err)
	}
	if updated.Name != "Updated Upload Model" || len(updated.Gallery) != 1 {
		t.Fatalf("unexpected updated model with uploads: %#v", updated)
	}

	entries, err := os.ReadDir(modelDir)
	if err != nil {
		t.Fatalf("read model image dir: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !slices.Contains(names, "cover.png") || !slices.Contains(names, "gallery_01.png") {
		t.Fatalf("unexpected files after update upload: %#v", names)
	}
	if slices.Contains(names, "gallery_02.png") {
		t.Fatalf("expected old gallery files to be cleaned up, got %#v", names)
	}

	if _, err := store.UpdateWithUploads(9999, models.CreateModelRequest{Name: "Missing"}, cover, nil); err == nil || !errorsIs(err, ErrModelNotFound) {
		t.Fatalf("expected ErrModelNotFound for missing update id, got: %v", err)
	}
}

func TestCleanupCreatedModelDir(t *testing.T) {
	root := t.TempDir()
	store := &ModelStore{imagesRoot: root}

	modelDir := modelImageDir(root, 42)
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("create model dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "cover.png"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write model image: %v", err)
	}

	store.cleanupCreatedModelDir(42, false)
	if _, err := os.Stat(modelDir); err != nil {
		t.Fatalf("expected directory to remain when cleanup disabled: %v", err)
	}

	store.cleanupCreatedModelDir(42, true)
	if _, err := os.Stat(modelDir); !os.IsNotExist(err) {
		t.Fatalf("expected model dir to be removed, got err=%v", err)
	}
}

func TestDeleteFilesByPrefix(t *testing.T) {
	dir := t.TempDir()
	files := []string{"cover.jpg", "cover.png", "gallery_01.png", "note.txt"}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	if err := deleteFilesByPrefix(dir, "cover"); err != nil {
		t.Fatalf("delete files by prefix: %v", err)
	}

	remaining, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	names := make([]string, 0, len(remaining))
	for _, entry := range remaining {
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"gallery_01.png", "note.txt"}) {
		t.Fatalf("unexpected remaining files: %#v", names)
	}
}

func TestLegacyParsingHelpers(t *testing.T) {
	if _, ok := parseLegacySeedModels([]byte("{bad-json"), "legacy.json"); ok {
		t.Fatal("expected invalid json to be skipped")
	}

	items, ok := parseLegacySeedModels([]byte(`[{"id":5,"name":"Legacy"}]`), "legacy.json")
	if !ok || len(items) != 1 {
		t.Fatalf("expected parsed legacy item, got ok=%v len=%d", ok, len(items))
	}

	state := newLegacyImportState(2)
	id1 := state.allocateID(10)
	id2 := state.allocateID(10)
	id3 := state.allocateID(0)
	if id1 != 10 || id2 == 10 || id3 <= 0 {
		t.Fatalf("unexpected allocated ids: %d, %d, %d", id1, id2, id3)
	}
}

func TestModelStoreCloseAndSeedEdgeCases(t *testing.T) {
	var nilStore *ModelStore
	if err := nilStore.Close(); err != nil {
		t.Fatalf("nil close should be no-op: %v", err)
	}

	store := newModelStoreForTest(t, "")
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("double close should not fail: %v", err)
	}

	store = newModelStoreForTest(t, "")
	if err := store.seedFromLegacyDataIfNeeded(); err != nil {
		t.Fatalf("seed should no-op when legacy path is empty: %v", err)
	}

	root := t.TempDir()
	legacyPath := filepath.Join(root, "legacy.json")
	store.legacyDataPath = legacyPath
	if err := store.seedFromLegacyDataIfNeeded(); err != nil {
		t.Fatalf("seed should no-op when legacy file is missing: %v", err)
	}

	if err := os.WriteFile(legacyPath, []byte(""), 0o644); err != nil {
		t.Fatalf("write empty legacy file: %v", err)
	}
	if err := store.seedFromLegacyDataIfNeeded(); err != nil {
		t.Fatalf("seed should no-op for empty legacy content: %v", err)
	}

	if err := os.WriteFile(legacyPath, []byte("{invalid-json"), 0o644); err != nil {
		t.Fatalf("write invalid legacy file: %v", err)
	}
	if err := store.seedFromLegacyDataIfNeeded(); err != nil {
		t.Fatalf("seed should skip parse errors without failing: %v", err)
	}
}

func TestDeleteFilesByPrefixReadDirErrorAndSaveUploadEdgeCases(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "not-a-dir.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := deleteFilesByPrefix(filePath, "cover"); err != nil {
		t.Fatalf("delete files should tolerate non-directory path, got: %v", err)
	}

	if _, err := saveUploadedImage(nil, root, "cover"); err == nil {
		t.Fatal("expected nil header error")
	}

	tooLarge := &multipart.FileHeader{
		Filename: "large.png",
		Size:     maxUploadBytes + 1,
	}
	if _, err := saveUploadedImage(tooLarge, root, "cover"); err == nil {
		t.Fatal("expected size limit error")
	}
}

func TestNewModelStoreErrorPaths(t *testing.T) {
	root := t.TempDir()

	dbParentAsFile := filepath.Join(root, "db-parent")
	if err := os.WriteFile(dbParentAsFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write db parent file: %v", err)
	}
	if _, err := NewModelStore(filepath.Join(dbParentAsFile, "models.db"), filepath.Join(root, "images"), ""); err == nil {
		t.Fatal("expected create db directory error")
	}

	imagesPathAsFile := filepath.Join(root, "images-as-file")
	if err := os.WriteFile(imagesPathAsFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write images path file: %v", err)
	}
	if _, err := NewModelStore(filepath.Join(root, "data", "models.db"), imagesPathAsFile, ""); err == nil {
		t.Fatal("expected create images directory error")
	}
}

func TestModelStoreAddAndUpdateUploadFailurePaths(t *testing.T) {
	store := newModelStoreForTest(t, "")

	tooLarge := &multipart.FileHeader{
		Filename: "cover.png",
		Size:     maxUploadBytes + 1,
	}
	_, err := store.AddWithUploads(models.CreateModelRequest{
		Name: "Bad Upload",
		Year: 2020,
	}, tooLarge, nil)
	if err == nil {
		t.Fatal("expected add with oversized cover to fail")
	}

	imageEntries, err := os.ReadDir(store.imagesRoot)
	if err != nil {
		t.Fatalf("read images root: %v", err)
	}
	if len(imageEntries) != 0 {
		t.Fatalf("expected cleanup after failed create upload, got entries=%d", len(imageEntries))
	}

	created, err := store.Add(models.CreateModelRequest{
		Name: "Existing Model",
		Year: 2020,
	})
	if err != nil {
		t.Fatalf("seed model: %v", err)
	}

	_, err = store.UpdateWithUploads(created.ID, models.CreateModelRequest{
		Name: "Existing Model",
		Year: 2020,
	}, tooLarge, nil)
	if err == nil {
		t.Fatal("expected update with oversized cover to fail")
	}
}

func TestModelStoreListAndPersistEdgeCases(t *testing.T) {
	store := newModelStoreForTest(t, "")

	nowText := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := store.db.Exec(`
		INSERT INTO car_models(
			name, model_code, brand, series, scale, year, color, material, condition,
			image_url, gallery_json, notes, tags_json, created_at, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "Broken", "", "", "", "", 0, "", "", "", "", "{bad-json", "", "[]", nowText, nowText)
	if err != nil {
		t.Fatalf("insert broken model row: %v", err)
	}

	items := store.List()
	if len(items) != 0 {
		t.Fatalf("expected invalid rows to be skipped, got %#v", items)
	}

	state := modelWriteState{
		now:      time.Now().UTC(),
		timeText: time.Now().UTC().Format(time.RFC3339Nano),
		imageURL: "/uploads/9/cover.png",
		gallery:  []string{"/uploads/9/a.png"},
		tags:     []string{"tag"},
	}
	err = store.persistModelUpdate(99999, models.CreateModelRequest{
		Name: "No Model",
		Year: 2021,
	}, state)
	if !errors.Is(err, ErrModelNotFound) {
		t.Fatalf("expected ErrModelNotFound, got: %v", err)
	}
}

func TestModelStoreClosedDBErrorPaths(t *testing.T) {
	store := newModelStoreForTest(t, "")
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	if err := store.initSchema(); err == nil {
		t.Fatal("expected initSchema to fail on closed db")
	}
	if _, err := store.isLegacySeedRequired(); err == nil {
		t.Fatal("expected isLegacySeedRequired to fail on closed db")
	}
	if err := store.importLegacyModels([]models.CarModel{{Name: "Legacy"}}); err == nil {
		t.Fatal("expected importLegacyModels to fail on closed db")
	}
}

func TestModelStoreWriteHelpersAndNoOpUploads(t *testing.T) {
	store := newModelStoreForTest(t, "")

	created, err := store.Add(models.CreateModelRequest{
		Name: "Write Helper Model",
		Year: 2022,
	})
	if err != nil {
		t.Fatalf("seed model: %v", err)
	}

	nowText := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := store.db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := updateModelImagesTx(tx, created.ID, "/uploads/1/cover.png", []string{"/uploads/1/a.png"}, nowText); err != nil {
		t.Fatalf("update model images tx: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	loaded, err := store.getByID(created.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if loaded.ImageURL != "/uploads/1/cover.png" || len(loaded.Gallery) != 1 {
		t.Fatalf("unexpected stored image fields: %#v", loaded)
	}

	tx2, err := store.db.Begin()
	if err != nil {
		t.Fatalf("begin tx2: %v", err)
	}
	if err := tx2.Rollback(); err != nil {
		t.Fatalf("rollback tx2: %v", err)
	}
	if _, err := store.insertModelTx(tx2, models.CreateModelRequest{Name: "X"}, modelWriteState{
		timeText: nowText,
		imageURL: "",
		gallery:  []string{},
		tags:     []string{},
	}); err == nil {
		t.Fatal("expected insertModelTx to fail on closed transaction")
	}
	if err := updateModelImagesTx(tx2, created.ID, "", nil, nowText); err == nil {
		t.Fatal("expected updateModelImagesTx to fail on closed transaction")
	}

	gallery := []string{"/uploads/1/a.png"}
	imageURL := "/uploads/1/cover.png"
	if err := applyGalleryUpload(created.ID, modelImageDir(store.imagesRoot, created.ID), nil, &gallery); err != nil {
		t.Fatalf("applyGalleryUpload with no files should be no-op: %v", err)
	}
	if err := replaceGalleryUpload(created.ID, modelImageDir(store.imagesRoot, created.ID), nil, &gallery, &imageURL); err != nil {
		t.Fatalf("replaceGalleryUpload with no files should be no-op: %v", err)
	}

	ensureImageURLFromGallery(&imageURL, []string{"/uploads/1/other.png"})
	if imageURL != "/uploads/1/cover.png" {
		t.Fatalf("expected existing image url to remain unchanged, got %s", imageURL)
	}

	emptyImageURL := ""
	ensureImageURLFromGallery(&emptyImageURL, []string{"/uploads/1/first.png"})
	if emptyImageURL != "/uploads/1/first.png" {
		t.Fatalf("expected empty image url to be filled from gallery, got %s", emptyImageURL)
	}
}

func TestNormalizeImageExtensionFallbackCases(t *testing.T) {
	cases := map[string]string{
		"image/gif":  ".gif",
		"image/webp": ".webp",
		"image/bmp":  ".bmp",
		"image/avif": ".avif",
	}

	for contentType, expected := range cases {
		if ext := normalizeImageExtension("model.unknown", contentType); ext != expected {
			t.Fatalf("content-type %s expected %s, got %s", contentType, expected, ext)
		}
	}
}

func errorsIs(err, target error) bool {
	return err != nil && target != nil && strings.Contains(err.Error(), target.Error())
}
