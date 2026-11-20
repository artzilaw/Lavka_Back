-- 000002_seed_initial_data.up.sql
-- Справочник категорий товаров

BEGIN;

INSERT INTO categories (id, title, slug) VALUES
    (1,  'Электроника',     'electronics'),
    (2,  'Авто и гаражи',   'auto'),
    (3,  'Недвижимость',    'realty'),
    (4,  'Одежда и обувь',  'clothes'),
    (5,  'Хобби и отдых',   'hobby'),
    (6,  'Для дома',        'home'),
    (7,  'Работа',          'jobs'),
    (8,  'Животные',        'pets'),
    (9,  'Запчасти',        'parts'),
    (10, 'Детям',           'kids'),
    (11, 'Красота',         'beauty')
ON CONFLICT (id) DO NOTHING;

SELECT setval(pg_get_serial_sequence('categories', 'id'), COALESCE((SELECT MAX(id) FROM categories), 1));

COMMIT;
