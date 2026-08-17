package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/internal/app/lostpet"
	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func beginLostPetImageUpload(
	t *testing.T,
	ctx context.Context,
	service *lostpet.Service,
	images *blob.MemoryBlobStore,
) *blob.ImageUploadGrant {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/lostPet/uploads", strings.NewReader(
		`{"purpose":"lost-pet","fileName":"buddy.png","contentType":"image/png"}`,
	))
	recorder := httptest.NewRecorder()
	service.HandleBeginImageUpload(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("lost image upload grant status = %d, want %d; body = %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	var grant blob.ImageUploadGrant
	if err := json.Unmarshal(recorder.Body.Bytes(), &grant); err != nil {
		t.Fatal(err)
	}
	if _, err := images.UploadImage(ctx, grant.ObjectName, e2eLostPetImage()); err != nil {
		t.Fatal(err)
	}
	return &grant
}

func reportLostPetWithImage(
	t *testing.T,
	service *lostpet.Service,
	grant *blob.ImageUploadGrant,
	report domain.LostPetReport,
) *httptest.ResponseRecorder {
	t.Helper()
	report.PetID = grant.ReportID
	report.ImageObject = grant.ObjectName
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/lostPet", bytes.NewReader(body))
	request.Header.Set("X-PetSpotR-Upload-Token", grant.FinalizeToken)
	recorder := httptest.NewRecorder()
	service.HandleLostPet(recorder, request)
	return recorder
}

func e2eLostPetImage() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0xf0,
		0x1f, 0x00, 0x05, 0x00, 0x01, 0xff, 0x89, 0x99,
		0x3d, 0x1d, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
		0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
}
