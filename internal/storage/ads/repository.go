package ads

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lavka/internal/domain/models"
)

var (
	ErrAdNotFound = errors.New("ad not found")
	ErrForbidden  = errors.New("forbidden")
)

type AdsRepository struct {
	mu     sync.RWMutex
	Data   []models.Ad
	nextID int64
	path   string
}

type adsDump struct {
	Ads    []models.Ad `json:"ads"`
	NextID int64       `json:"next_id"`
}

func NewAdsRepository() *AdsRepository {
	initial := generateMockAds()
	var maxID int64
	for _, a := range initial {
		if a.ID > maxID {
			maxID = a.ID
		}
	}
	return &AdsRepository{
		Data:   initial,
		nextID: maxID,
	}
}

func NewAdsRepositoryWithFile(path string) (*AdsRepository, error) {
	r := NewAdsRepository()
	r.path = path

	if path == "" {
		return r, nil
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(path); err == nil {
		if err := r.load(); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	} else {
		if err := r.persist(); err != nil {
			return nil, err
		}
	}

	return r, nil
}

func (r *AdsRepository) load() error {
	f, err := os.Open(r.path)
	if err != nil {
		return err
	}
	defer f.Close()

	var d adsDump
	if err := json.NewDecoder(f).Decode(&d); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}

	if len(d.Ads) > 0 {
		r.Data = d.Ads
	}
	if d.NextID > r.nextID {
		r.nextID = d.NextID
	}
	for _, a := range r.Data {
		if a.ID > r.nextID {
			r.nextID = a.ID
		}
	}
	return nil
}

func (r *AdsRepository) persist() error {
	if r.path == "" {
		return nil
	}

	d := adsDump{
		Ads:    r.Data,
		NextID: r.nextID,
	}

	f, err := os.Create(r.path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(&d)
}

func cloneAd(ad models.Ad) models.Ad {
	if ad.Photos != nil {
		photos := make([]string, len(ad.Photos))
		copy(photos, ad.Photos)
		ad.Photos = photos
	}
	return ad
}

func (r *AdsRepository) GetAll() []models.Ad {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]models.Ad, 0, len(r.Data))
	for _, ad := range r.Data {
		if ad.Status == models.AdStatusActive {
			result = append(result, cloneAd(ad))
		}
	}
	return result
}

func (r *AdsRepository) GetByID(id int64) (models.Ad, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, ad := range r.Data {
		if ad.ID == id && ad.Status != models.AdStatusDeleted {
			return cloneAd(ad), nil
		}
	}
	return models.Ad{}, ErrAdNotFound
}

func (r *AdsRepository) IncrementViews(id int64) (models.Ad, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.Data {
		if r.Data[i].ID == id && r.Data[i].Status != models.AdStatusDeleted {
			r.Data[i].ViewsCount++
			_ = r.persist()
			return cloneAd(r.Data[i]), nil
		}
	}
	return models.Ad{}, ErrAdNotFound
}

func (r *AdsRepository) GetBySellerID(sellerID int64, includeClosed bool) []models.Ad {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]models.Ad, 0)
	for _, ad := range r.Data {
		if ad.SellerID != sellerID || ad.Status == models.AdStatusDeleted {
			continue
		}
		if !includeClosed && ad.Status != models.AdStatusActive {
			continue
		}
		result = append(result, cloneAd(ad))
	}
	return result
}

func (r *AdsRepository) CountBySeller(sellerID int64) (active int, closed int) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, ad := range r.Data {
		if ad.SellerID != sellerID || ad.Status == models.AdStatusDeleted {
			continue
		}
		if ad.Status == models.AdStatusActive {
			active++
		} else {
			closed++
		}
	}
	return active, closed
}

func (r *AdsRepository) Create(ad models.Ad) (models.Ad, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID++
	now := time.Now()
	ad.ID = r.nextID
	ad.Status = models.AdStatusActive
	ad.ViewsCount = 0
	ad.FavoritesCount = 0
	ad.CreatedAt = now
	ad.UpdatedAt = now
	if ad.Photos == nil {
		ad.Photos = []string{}
	}

	r.Data = append([]models.Ad{cloneAd(ad)}, r.Data...)
	if err := r.persist(); err != nil {
		return models.Ad{}, err
	}
	return cloneAd(ad), nil
}

func (r *AdsRepository) Update(id int64, sellerID int64, patch models.Ad) (models.Ad, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.Data {
		if r.Data[i].ID == id && r.Data[i].Status != models.AdStatusDeleted {
			if r.Data[i].SellerID != sellerID {
				return models.Ad{}, ErrForbidden
			}
			r.Data[i].Title = patch.Title
			r.Data[i].Description = patch.Description
			r.Data[i].Price = patch.Price
			r.Data[i].CategoryID = patch.CategoryID
			r.Data[i].Location = patch.Location
			if patch.Photos != nil {
				photos := make([]string, len(patch.Photos))
				copy(photos, patch.Photos)
				r.Data[i].Photos = photos
			}
			r.Data[i].UpdatedAt = time.Now()
			if err := r.persist(); err != nil {
				return models.Ad{}, err
			}
			return cloneAd(r.Data[i]), nil
		}
	}
	return models.Ad{}, ErrAdNotFound
}

func (r *AdsRepository) Close(id int64, sellerID int64) (models.Ad, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.Data {
		if r.Data[i].ID == id && r.Data[i].Status != models.AdStatusDeleted {
			if r.Data[i].SellerID != sellerID {
				return models.Ad{}, ErrForbidden
			}
			r.Data[i].Status = models.AdStatusClosed
			r.Data[i].UpdatedAt = time.Now()
			if err := r.persist(); err != nil {
				return models.Ad{}, err
			}
			return cloneAd(r.Data[i]), nil
		}
	}
	return models.Ad{}, ErrAdNotFound
}

func (r *AdsRepository) Reopen(id int64, sellerID int64) (models.Ad, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.Data {
		if r.Data[i].ID == id && r.Data[i].Status != models.AdStatusDeleted {
			if r.Data[i].SellerID != sellerID {
				return models.Ad{}, ErrForbidden
			}
			r.Data[i].Status = models.AdStatusActive
			r.Data[i].UpdatedAt = time.Now()
			if err := r.persist(); err != nil {
				return models.Ad{}, err
			}
			return cloneAd(r.Data[i]), nil
		}
	}
	return models.Ad{}, ErrAdNotFound
}

func (r *AdsRepository) MarkSold(ids []int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	idSet := make(map[int64]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}

	now := time.Now()
	changed := false
	for i := range r.Data {
		if idSet[r.Data[i].ID] && r.Data[i].Status != models.AdStatusDeleted {
			r.Data[i].Status = models.AdStatusSold
			r.Data[i].UpdatedAt = now
			changed = true
		}
	}

	if changed {
		return r.persist()
	}
	return nil
}

func (r *AdsRepository) Delete(id int64, sellerID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.Data {
		if r.Data[i].ID == id && r.Data[i].Status != models.AdStatusDeleted {
			if r.Data[i].SellerID != sellerID {
				return ErrForbidden
			}
			now := time.Now()
			r.Data[i].Status = models.AdStatusDeleted
			r.Data[i].DeletedAt = now
			r.Data[i].UpdatedAt = now
			return r.persist()
		}
	}
	return ErrAdNotFound
}

func generateMockAds() []models.Ad {
	now := time.Now()

	return []models.Ad{
		{
			ID:             1,
			SellerID:       101,
			CategoryID:     1,
			Title:          "MacBook Pro 16 M1 Max",
			Description:    "Отличное состояние, полный комплект, использовался только для программирования. Батарея 95%.",
			Price:          250000,
			Status:         models.AdStatusActive,
			Location:       "Москва",
			Photos:         []string{"/static/img/1.webp"},
			ViewsCount:     150,
			FavoritesCount: 15,
			CreatedAt:      now.Add(-24 * time.Hour),
		},
		{
			ID:             2,
			SellerID:       102,
			CategoryID:     6,
			Title:          "Диван-кровать Икеа",
			Description:    "Хорошее состояние, есть небольшие пятна на обивке. Только самовывоз.",
			Price:          50000,
			Status:         models.AdStatusActive,
			Location:       "Санкт-Петербург",
			Photos:         []string{"/static/img/2.webp"},
			ViewsCount:     45,
			FavoritesCount: 3,
			CreatedAt:      now.Add(-48 * time.Hour),
		},
		{
			ID:             3,
			SellerID:       103,
			CategoryID:     5,
			Title:          "Горный велосипед Stern",
			Description:    "Почти новый, катались всего пару раз. Рама 20 дюймов, колеса 27.5.",
			Price:          55000,
			Status:         models.AdStatusActive,
			Location:       "Казань",
			Photos:         []string{"/static/img/3.webp"},
			ViewsCount:     12,
			FavoritesCount: 1,
			CreatedAt:      now.Add(-72 * time.Hour),
		},
		{
			ID:             4,
			SellerID:       104,
			CategoryID:     5,
			Title:          "Комплект книг Гарри Поттер",
			Description:    "Полное собрание от издательства РОСМЭН. Идеальное состояние, как новые.",
			Price:          0,
			Status:         models.AdStatusActive,
			Location:       "Новосибирск",
			Photos:         []string{"/static/img/4.webp"},
			ViewsCount:     300,
			FavoritesCount: 42,
			CreatedAt:      now.Add(-2 * time.Hour),
		},
		{
			ID:             5,
			SellerID:       105,
			CategoryID:     1,
			Title:          "iPhone 15 Pro Max 256GB",
			Description:    "Новый, запечатанный. Цвет Natural Titanium. Официальная гарантия.",
			Price:          125000,
			Status:         models.AdStatusActive,
			Location:       "Москва",
			Photos:         []string{"/static/img/5.webp", "/static/img/6.webp"},
			ViewsCount:     890,
			FavoritesCount: 112,
			CreatedAt:      now.Add(-1 * time.Hour),
		},
		{
			ID:             6,
			SellerID:       106,
			CategoryID:     6,
			Title:          "Игровое кресло Cougar",
			Description:    "Эко-кожа, стальная рама, регулировка 4D подлокотников. Использовалось полгода.",
			Price:          18500,
			Status:         models.AdStatusActive,
			Location:       "Екатеринбург",
			Photos:         []string{"/static/img/7.webp"},
			ViewsCount:     210,
			FavoritesCount: 18,
			CreatedAt:      now.Add(-10 * time.Hour),
		},
		{
			ID:             7,
			SellerID:       107,
			CategoryID:     5,
			Title:          "Гитара Fender Stratocaster",
			Description:    "Made in Mexico, 2019 год. Шикарный звук, новые струны Ernie Ball.",
			Price:          85000,
			Status:         models.AdStatusActive,
			Location:       "Краснодар",
			Photos:         []string{"/static/img/8.webp", "/static/img/9.webp"},
			ViewsCount:     345,
			FavoritesCount: 56,
			CreatedAt:      now.Add(-5 * time.Hour),
		},
		{
			ID:             8,
			SellerID:       108,
			CategoryID:     9,
			Title:          "Набор инструментов 120 предметов",
			Description:    "Сталь хром-ванадий. Пожизненная гарантия. Почти не пользовались.",
			Price:          7500,
			Status:         models.AdStatusActive,
			Location:       "Нижний Новгород",
			Photos:         []string{"/static/img/10.webp"},
			ViewsCount:     76,
			FavoritesCount: 12,
			CreatedAt:      now.Add(-15 * time.Hour),
		},
		{
			ID:             9,
			SellerID:       109,
			CategoryID:     1,
			Title:          "Sony PlayStation 5",
			Description:    "Версия с дисководом, 2 геймпада DualSense и игра в подарок.",
			Price:          49000,
			Status:         models.AdStatusActive,
			Location:       "Владивосток",
			Photos:         []string{"/static/img/11.webp"},
			ViewsCount:     560,
			FavoritesCount: 98,
			CreatedAt:      now.Add(-3 * time.Hour),
		},
		{
			ID:             10,
			SellerID:       110,
			CategoryID:     6,
			Title:          "Ковер персидский ручной работы",
			Description:    "Натуральная шерсть. Размер 2х3 метра. Состояние идеальное.",
			Price:          30000,
			Status:         models.AdStatusActive,
			Location:       "Махачкала",
			Photos:         []string{"/static/img/12.webp"},
			ViewsCount:     33,
			FavoritesCount: 5,
			CreatedAt:      now.Add(-96 * time.Hour),
		},
		{
			ID:             11,
			SellerID:       111,
			CategoryID:     5,
			Title:          "Коллекционное издание Булгакова",
			Description:    "Кожаный переплет, золотое тиснение. Ограниченная серия.",
			Price:          15000,
			Status:         models.AdStatusActive,
			Location:       "Тула",
			Photos:         []string{"/static/img/13.webp"},
			ViewsCount:     82,
			FavoritesCount: 21,
			CreatedAt:      now.Add(-20 * time.Hour),
		},
		{
			ID:             12,
			SellerID:       112,
			CategoryID:     5,
			Title:          "Палатка 3-х местная Tramp",
			Description:    "Двухслойная, влагозащита 5000мм. Идеальна для походов в горы.",
			Price:          12000,
			Status:         models.AdStatusActive,
			Location:       "Сочи",
			Photos:         []string{"/static/img/14.webp"},
			ViewsCount:     140,
			FavoritesCount: 25,
			CreatedAt:      now.Add(-12 * time.Hour),
		},
	}
}
