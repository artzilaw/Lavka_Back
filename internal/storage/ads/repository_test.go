package ads

import "testing"

func TestAdsRepository_GetAll(t *testing.T) {
	repo := NewAdsRepository()

	ads := repo.GetAll()
	if len(ads) == 0 {
		t.Fatal("expected non-empty ads list")
	}

	originalTitle := ads[0].Title
	ads[0].Title = "Modified Title"

	adsAgain := repo.GetAll()
	if adsAgain[0].Title != originalTitle {
		t.Errorf("repository data was mutated: got %q, want %q", adsAgain[0].Title, originalTitle)
	}
}
