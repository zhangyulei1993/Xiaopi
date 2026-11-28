package httpapi

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xiaopi/backend/internal/domain"
)

func TestCommittedMediaFilesAreRetainedWhenRecordsChange(t *testing.T) {
	for _, operation := range []string{"replace", "delete-property", "delete-material"} {
		t.Run(operation, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("FILE_STORAGE_LOCAL_PATH", dir)
			file := filepath.Join(dir, "shared.jpg")
			if err := os.WriteFile(file, []byte("media"), 0600); err != nil {
				t.Fatal(err)
			}
			item := domain.Property{ID: "one", Title: "test", IsMaterial: operation == "delete-material", Media: []domain.PropertyMedia{{ID: "media-one", URL: "/uploads/shared.jpg"}}}
			server := NewServerWithConfig(&fakeStore{properties: []domain.Property{item}}, Config{})
			var req *httptest.ResponseRecorder
			switch operation {
			case "replace":
				req = httptest.NewRecorder()
				server.replacePropertyMedia(req, httptest.NewRequest("POST", "/", strings.NewReader(`{"propertyId":"one","mediaIds":[]}`)))
			case "delete-property":
				req = httptest.NewRecorder()
				request := httptest.NewRequest("DELETE", "/", nil)
				request.SetPathValue("propertyID", "one")
				server.deleteProperty(req, request)
			case "delete-material":
				req = httptest.NewRecorder()
				request := httptest.NewRequest("DELETE", "/", nil)
				request.SetPathValue("propertyID", "one")
				server.deleteMaterial(req, request)
			}
			if req.Code != 200 {
				t.Fatalf("unexpected status: %d %s", req.Code, req.Body.String())
			}
			if _, err := os.Stat(file); err != nil {
				t.Fatalf("committed media was removed: %v", err)
			}
		})
	}
}
