package ads

import (
	"sync"
	"time"

	"lavka/internal/domain/models"
)

type AdsRepository struct {
	mu   sync.RWMutex
	Data []models.Ad
}

func NewAdsRepository() *AdsRepository {
	return &AdsRepository{
		Data: generateMockAds(),
	}
}

func (r *AdsRepository) GetAll() []models.Ad {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]models.Ad, len(r.Data))
	copy(result, r.Data)
	return result
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
			Status:         "active",
			Location:       "Москва",
			Photos:         []string{"/static/img/1.webp"},
			ViewsCount:     150,
			FavoritesCount: 15,
			CreatedAt:      now.Add(-24 * time.Hour),
		},
		{
			ID:             2,
			SellerID:       102,
			CategoryID:     2,
			Title:          "Диван-кровать Икеа",
			Description:    "Хорошее состояние, есть небольшие пятна на обивке. Только самовывоз.",
			Price:          50000,
			Status:         "active",
			Location:       "Санкт-Петербург",
			Photos:         []string{"/static/img/2.webp"},
			ViewsCount:     45,
			FavoritesCount: 3,
			CreatedAt:      now.Add(-48 * time.Hour),
		},
		{
			ID:             3,
			SellerID:       103,
			CategoryID:     3,
			Title:          "Горный велосипед Stern",
			Description:    "Почти новый, катались всего пару раз. Рама 20 дюймов, колеса 27.5.",
			Price:          55000,
			Status:         "active",
			Location:       "Казань",
			Photos:         []string{"/static/img/3.webp"},
			ViewsCount:     12,
			FavoritesCount: 1,
			CreatedAt:      now.Add(-72 * time.Hour),
		},
		{
			ID:             4,
			SellerID:       104,
			CategoryID:     4,
			Title:          "Комплект книг Гарри Поттер",
			Description:    "Полное собрание от издательства РОСМЭН. Идеальное состояние, как новые.",
			Price:          0,
			Status:         "active",
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
			Status:         "active",
			Location:       "Москва",
			Photos:         []string{"/static/img/5.webp", "/static/img/6.webp"},
			ViewsCount:     890,
			FavoritesCount: 112,
			CreatedAt:      now.Add(-1 * time.Hour),
		},
		{
			ID:             6,
			SellerID:       106,
			CategoryID:     2,
			Title:          "Игровое кресло Cougar",
			Description:    "Эко-кожа, стальная рама, регулировка 4D подлокотников. Использовалось полгода.",
			Price:          18500,
			Status:         "active",
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
			Status:         "active",
			Location:       "Краснодар",
			Photos:         []string{"/static/img/8.webp", "/static/img/9.webp"},
			ViewsCount:     345,
			FavoritesCount: 56,
			CreatedAt:      now.Add(-5 * time.Hour),
		},
		{
			ID:             8,
			SellerID:       108,
			CategoryID:     6,
			Title:          "Набор инструментов 120 предметов",
			Description:    "Сталь хром-ванадий. Пожизненная гарантия. Почти не пользовались.",
			Price:          7500,
			Status:         "active",
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
			Status:         "active",
			Location:       "Владивосток",
			Photos:         []string{"/static/img/11.webp"},
			ViewsCount:     560,
			FavoritesCount: 98,
			CreatedAt:      now.Add(-3 * time.Hour),
		},
		{
			ID:             10,
			SellerID:       110,
			CategoryID:     2,
			Title:          "Ковер персидский ручной работы",
			Description:    "Натуральная шерсть. Размер 2х3 метра. Состояние идеальное.",
			Price:          30000,
			Status:         "active",
			Location:       "Махачкала",
			Photos:         []string{"/static/img/12.webp"},
			ViewsCount:     33,
			FavoritesCount: 5,
			CreatedAt:      now.Add(-96 * time.Hour),
		},
		{
			ID:             11,
			SellerID:       111,
			CategoryID:     4,
			Title:          "Коллекционное издание Булгакова",
			Description:    "Кожаный переплет, золотое тиснение. Ограниченная серия.",
			Price:          15000,
			Status:         "active",
			Location:       "Тула",
			Photos:         []string{"/static/img/13.webp"},
			ViewsCount:     82,
			FavoritesCount: 21,
			CreatedAt:      now.Add(-20 * time.Hour),
		},
		{
			ID:             12,
			SellerID:       112,
			CategoryID:     7,
			Title:          "Палатка 3-х местная Tramp",
			Description:    "Двухслойная, влагозащита 5000мм. Идеальна для походов в горы.",
			Price:          12000,
			Status:         "active",
			Location:       "Сочи",
			Photos:         []string{"/static/img/14.webp"},
			ViewsCount:     140,
			FavoritesCount: 25,
			CreatedAt:      now.Add(-12 * time.Hour),
		},
	}
}
