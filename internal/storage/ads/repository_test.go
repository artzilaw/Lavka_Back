package ads

import (
	"errors"
	"path/filepath"
	"testing"

	"lavka/internal/domain/models"
)

func TestAdsRepository_GetAll(t *testing.T) {
	repo := NewAdsRepository()

	adsList := repo.GetAll()
	if len(adsList) == 0 {
		t.Fatal("expected non-empty ads list")
	}

	originalTitle := adsList[0].Title
	adsList[0].Title = "Modified Title"

	adsAgain := repo.GetAll()
	if adsAgain[0].Title != originalTitle {
		t.Errorf("repository data was mutated: got %q, want %q", adsAgain[0].Title, originalTitle)
	}
}

func TestAdsRepository_CRUDAndLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ads.json")
	repo, err := NewAdsRepositoryWithFile(path)
	if err != nil {
		t.Fatalf("NewAdsRepositoryWithFile error: %v", err)
	}

	created, err := repo.Create(models.Ad{
		SellerID:    42,
		CategoryID:  1,
		Title:       "Test Laptop",
		Description: "Great laptop in mint condition",
		Price:       99000,
		Location:    "Москва",
		Photos:      []string{"/static/img/1.webp"},
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if created.ID == 0 || created.Status != models.AdStatusActive {
		t.Fatalf("unexpected created ad: %+v", created)
	}

	viewed, err := repo.IncrementViews(created.ID)
	if err != nil || viewed.ViewsCount != 1 {
		t.Fatalf("IncrementViews failed: ad=%+v err=%v", viewed, err)
	}

	// Non-owner update should fail
	if _, err := repo.Update(created.ID, 999, created); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for non-owner update, got %v", err)
	}

	updated, err := repo.Update(created.ID, 42, models.Ad{
		CategoryID:  1,
		Title:       "Updated Laptop",
		Description: "Updated description here",
		Price:       89000,
		Location:    "Казань",
		Photos:      []string{"/static/img/2.webp"},
	})
	if err != nil || updated.Title != "Updated Laptop" || updated.Price != 89000 {
		t.Fatalf("Update failed: ad=%+v err=%v", updated, err)
	}

	closed, err := repo.Close(created.ID, 42)
	if err != nil || closed.Status != models.AdStatusClosed {
		t.Fatalf("Close failed: ad=%+v err=%v", closed, err)
	}

	active, closedCnt := repo.CountBySeller(42)
	if active != 0 || closedCnt != 1 {
		t.Fatalf("CountBySeller = (%d, %d), want (0, 1)", active, closedCnt)
	}

	reopened, err := repo.Reopen(created.ID, 42)
	if err != nil || reopened.Status != models.AdStatusActive {
		t.Fatalf("Reopen failed: ad=%+v err=%v", reopened, err)
	}

	if err := repo.Delete(created.ID, 42); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := repo.GetByID(created.ID); !errors.Is(err, ErrAdNotFound) {
		t.Fatalf("expected ErrAdNotFound after delete, got %v", err)
	}
}
