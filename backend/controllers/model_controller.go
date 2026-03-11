package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/beego/beego/v2/server/web"

	"bigtoy/backend/models"
	"bigtoy/backend/services"
)

const maxModelUploadBytes = 64 << 20

var modelStore *services.ModelStore

func SetModelStore(store *services.ModelStore) {
	modelStore = store
}

type ModelController struct {
	web.Controller
}

func (c *ModelController) Get() {
	if modelStore == nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]any{"error": "model store is not initialized"}
		c.ServeJSON()
		return
	}

	items := modelStore.List()
	c.Data["json"] = map[string]any{
		"data":  items,
		"total": len(items),
	}
	c.ServeJSON()
}

func (c *ModelController) Post() {
	if modelStore == nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]any{"error": "model store is not initialized"}
		c.ServeJSON()
		return
	}
	if _, ok := sessionFromRequest(c.Ctx.Request); !ok {
		c.Ctx.Output.SetStatus(http.StatusUnauthorized)
		c.Data["json"] = map[string]any{"error": "authentication required"}
		c.ServeJSON()
		return
	}

	req, coverFile, galleryFiles, err := parseCreateModelRequest(c)
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.Data["json"] = map[string]any{"error": err.Error()}
		c.ServeJSON()
		return
	}
	if c.Ctx.Request.MultipartForm != nil {
		defer c.Ctx.Request.MultipartForm.RemoveAll()
	}

	var item models.CarModel
	if coverFile != nil || len(galleryFiles) > 0 {
		item, err = modelStore.AddWithUploads(req, coverFile, galleryFiles)
	} else {
		item, err = modelStore.Add(req)
	}
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.Data["json"] = map[string]any{"error": err.Error()}
		c.ServeJSON()
		return
	}

	c.Ctx.Output.SetStatus(http.StatusCreated)
	c.Data["json"] = map[string]any{"data": item}
	c.ServeJSON()
}

func (c *ModelController) Put() {
	if modelStore == nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]any{"error": "model store is not initialized"}
		c.ServeJSON()
		return
	}
	if _, ok := sessionFromRequest(c.Ctx.Request); !ok {
		c.Ctx.Output.SetStatus(http.StatusUnauthorized)
		c.Data["json"] = map[string]any{"error": "authentication required"}
		c.ServeJSON()
		return
	}

	modelID, err := parseModelID(c)
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.Data["json"] = map[string]any{"error": err.Error()}
		c.ServeJSON()
		return
	}

	req, coverFile, galleryFiles, err := parseCreateModelRequest(c)
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.Data["json"] = map[string]any{"error": err.Error()}
		c.ServeJSON()
		return
	}
	if c.Ctx.Request.MultipartForm != nil {
		defer c.Ctx.Request.MultipartForm.RemoveAll()
	}

	var item models.CarModel
	if coverFile != nil || len(galleryFiles) > 0 {
		item, err = modelStore.UpdateWithUploads(modelID, req, coverFile, galleryFiles)
	} else {
		item, err = modelStore.Update(modelID, req)
	}
	if err != nil {
		switch {
		case errors.Is(err, services.ErrModelNotFound):
			c.Ctx.Output.SetStatus(http.StatusNotFound)
		default:
			c.Ctx.Output.SetStatus(http.StatusBadRequest)
		}
		c.Data["json"] = map[string]any{"error": err.Error()}
		c.ServeJSON()
		return
	}

	c.Data["json"] = map[string]any{"data": item}
	c.ServeJSON()
}

func (c *ModelController) Delete() {
	if modelStore == nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]any{"error": "model store is not initialized"}
		c.ServeJSON()
		return
	}
	if _, ok := sessionFromRequest(c.Ctx.Request); !ok {
		c.Ctx.Output.SetStatus(http.StatusUnauthorized)
		c.Data["json"] = map[string]any{"error": "authentication required"}
		c.ServeJSON()
		return
	}

	modelID, err := parseModelID(c)
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.Data["json"] = map[string]any{"error": err.Error()}
		c.ServeJSON()
		return
	}

	if err := modelStore.Delete(modelID); err != nil {
		switch {
		case errors.Is(err, services.ErrModelNotFound):
			c.Ctx.Output.SetStatus(http.StatusNotFound)
		default:
			c.Ctx.Output.SetStatus(http.StatusBadRequest)
		}
		c.Data["json"] = map[string]any{"error": err.Error()}
		c.ServeJSON()
		return
	}

	c.Data["json"] = map[string]any{
		"data": map[string]any{
			"deleted": true,
			"id":      modelID,
		},
	}
	c.ServeJSON()
}

func parseCreateModelRequest(c *ModelController) (models.CreateModelRequest, *multipart.FileHeader, []*multipart.FileHeader, error) {
	request := c.Ctx.Request
	contentType := strings.ToLower(strings.TrimSpace(request.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		return parseMultipartModelRequest(request)
	}

	var req models.CreateModelRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		return models.CreateModelRequest{}, nil, nil, fmt.Errorf("invalid JSON payload")
	}
	return req, nil, nil, nil
}

func parseMultipartModelRequest(request *http.Request) (models.CreateModelRequest, *multipart.FileHeader, []*multipart.FileHeader, error) {
	if err := request.ParseMultipartForm(maxModelUploadBytes); err != nil {
		return models.CreateModelRequest{}, nil, nil, fmt.Errorf("invalid multipart payload")
	}

	req := models.CreateModelRequest{
		Name:      strings.TrimSpace(request.FormValue("name")),
		ModelCode: strings.TrimSpace(request.FormValue("modelCode")),
		Brand:     strings.TrimSpace(request.FormValue("brand")),
		Series:    strings.TrimSpace(request.FormValue("series")),
		Scale:     strings.TrimSpace(request.FormValue("scale")),
		Year:      parseIntOrZero(request.FormValue("year")),
		Color:     strings.TrimSpace(request.FormValue("color")),
		Material:  strings.TrimSpace(request.FormValue("material")),
		Condition: strings.TrimSpace(request.FormValue("condition")),
		ImageURL:  strings.TrimSpace(request.FormValue("imageUrl")),
		Notes:     strings.TrimSpace(request.FormValue("notes")),
		Tags:      splitByComma(request.FormValue("tags")),
		Gallery:   splitByCommaOrNewline(request.FormValue("gallery")),
	}

	coverFile := firstValidFile(request.MultipartForm.File["imageFile"])
	galleryFiles := validFiles(request.MultipartForm.File["galleryFiles"])
	return req, coverFile, galleryFiles, nil
}

func parseIntOrZero(raw string) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func splitByComma(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}

func splitByCommaOrNewline(raw string) []string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r", ""), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		parts := strings.Split(line, ",")
		for _, part := range parts {
			value := strings.TrimSpace(part)
			if value == "" {
				continue
			}
			result = append(result, value)
		}
	}
	return result
}

func firstValidFile(files []*multipart.FileHeader) *multipart.FileHeader {
	for _, file := range files {
		if file == nil {
			continue
		}
		if strings.TrimSpace(file.Filename) == "" {
			continue
		}
		if file.Size <= 0 {
			continue
		}
		return file
	}
	return nil
}

func validFiles(files []*multipart.FileHeader) []*multipart.FileHeader {
	result := make([]*multipart.FileHeader, 0, len(files))
	for _, file := range files {
		if file == nil {
			continue
		}
		if strings.TrimSpace(file.Filename) == "" {
			continue
		}
		if file.Size <= 0 {
			continue
		}
		result = append(result, file)
	}
	return result
}

func parseModelID(c *ModelController) (int64, error) {
	rawID := strings.TrimSpace(c.Ctx.Input.Param(":id"))
	if rawID == "" {
		return 0, fmt.Errorf("model id is required")
	}

	modelID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || modelID <= 0 {
		return 0, fmt.Errorf("invalid model id")
	}

	return modelID, nil
}
