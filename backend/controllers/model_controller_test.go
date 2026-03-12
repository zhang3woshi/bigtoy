package controllers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/beego/beego/v2/server/web/context"
)

func newModelControllerForTest(method, target string, body []byte, contentType string) *ModelController {
	request := httptest.NewRequest(method, target, bytes.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}

	writer := httptest.NewRecorder()
	ctx := context.NewContext()
	ctx.Reset(writer, request)

	controller := &ModelController{}
	controller.Ctx = ctx
	controller.Ctx.Input.RequestBody = body
	return controller
}

func TestParseIntOrZero(t *testing.T) {
	if value := parseIntOrZero("42"); value != 42 {
		t.Fatalf("expected 42, got %d", value)
	}
	if value := parseIntOrZero(" not-number "); value != 0 {
		t.Fatalf("expected 0 for invalid number, got %d", value)
	}
	if value := parseIntOrZero(""); value != 0 {
		t.Fatalf("expected 0 for empty value, got %d", value)
	}
}

func TestSplitHelpers(t *testing.T) {
	parts := splitByComma("a, b, ,c")
	if !slicesEqual(parts, []string{"a", "b", "c"}) {
		t.Fatalf("unexpected splitByComma result: %#v", parts)
	}

	lines := splitByCommaOrNewline("a, b\r\nc\n \n d,e")
	if !slicesEqual(lines, []string{"a", "b", "c", "d", "e"}) {
		t.Fatalf("unexpected splitByCommaOrNewline result: %#v", lines)
	}
}

func TestFileFilters(t *testing.T) {
	files := []*multipart.FileHeader{
		nil,
		{Filename: "", Size: 10},
		{Filename: "empty.png", Size: 0},
		{Filename: "cover.png", Size: 100},
	}

	first := firstValidFile(files)
	if first == nil || first.Filename != "cover.png" {
		t.Fatalf("expected first valid file cover.png, got %#v", first)
	}

	valid := validFiles(files)
	if len(valid) != 1 || valid[0].Filename != "cover.png" {
		t.Fatalf("unexpected valid files: %#v", valid)
	}
}

func TestParseModelID(t *testing.T) {
	controller := newModelControllerForTest(http.MethodPut, "/api/models/7", nil, "application/json")
	controller.Ctx.Input.SetParam(":id", "7")

	id, err := parseModelID(controller)
	if err != nil {
		t.Fatalf("parse model id should succeed: %v", err)
	}
	if id != 7 {
		t.Fatalf("expected id 7, got %d", id)
	}

	controller.Ctx.Input.SetParam(":id", "")
	if _, err := parseModelID(controller); err == nil {
		t.Fatal("expected missing id error")
	}

	controller.Ctx.Input.SetParam(":id", "abc")
	if _, err := parseModelID(controller); err == nil {
		t.Fatal("expected invalid id error")
	}
}

func TestParseCreateModelRequestJSON(t *testing.T) {
	body := []byte(`{"name":" Skyline ","year":1998,"tags":["jdm"]}`)
	controller := newModelControllerForTest(http.MethodPost, "/api/models", body, "application/json")

	req, cover, gallery, err := parseCreateModelRequest(controller)
	if err != nil {
		t.Fatalf("parse json request: %v", err)
	}
	if cover != nil || len(gallery) != 0 {
		t.Fatalf("expected no upload files, got cover=%v gallery=%d", cover, len(gallery))
	}
	if req.Name != " Skyline " || req.Year != 1998 {
		t.Fatalf("unexpected parsed json request: %#v", req)
	}

	badController := newModelControllerForTest(http.MethodPost, "/api/models", []byte("{bad"), "application/json")
	if _, _, _, err := parseCreateModelRequest(badController); err == nil {
		t.Fatal("expected invalid json error")
	}
}

func TestParseMultipartModelRequest(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	_ = writer.WriteField("name", " Skyline ")
	_ = writer.WriteField("modelCode", " BNR34 ")
	_ = writer.WriteField("brand", " Nissan ")
	_ = writer.WriteField("series", " GT-R ")
	_ = writer.WriteField("scale", " 1:64 ")
	_ = writer.WriteField("year", "1999")
	_ = writer.WriteField("color", " blue ")
	_ = writer.WriteField("material", " metal ")
	_ = writer.WriteField("condition", " mint ")
	_ = writer.WriteField("imageUrl", " /uploads/1/cover.png ")
	_ = writer.WriteField("notes", " iconic ")
	_ = writer.WriteField("tags", "jdm, nissan")
	_ = writer.WriteField("gallery", "a.png, b.png\nc.png")

	coverPart, err := writer.CreateFormFile("imageFile", "cover.png")
	if err != nil {
		t.Fatalf("create cover file: %v", err)
	}
	_, _ = coverPart.Write([]byte("cover-bytes"))

	galleryPart, err := writer.CreateFormFile("galleryFiles", "gallery01.png")
	if err != nil {
		t.Fatalf("create gallery file: %v", err)
	}
	_, _ = galleryPart.Write([]byte("gallery-bytes"))

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/models", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	req, coverFile, galleryFiles, err := parseMultipartModelRequest(request)
	if err != nil {
		t.Fatalf("parse multipart request: %v", err)
	}

	if strings.TrimSpace(req.Name) != "Skyline" || req.Year != 1999 {
		t.Fatalf("unexpected parsed multipart model: %#v", req)
	}
	if coverFile == nil || coverFile.Filename != "cover.png" {
		t.Fatalf("unexpected cover file: %#v", coverFile)
	}
	if len(galleryFiles) != 1 || galleryFiles[0].Filename != "gallery01.png" {
		t.Fatalf("unexpected gallery files: %#v", galleryFiles)
	}
}

func slicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
