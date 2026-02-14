-- Migration UP: 3 ta dorixona uchun jadvallarni yaratish
-- Bu fayl database'ni yangi versiyaga ko'taradi

-- 1. Eski jadvallarni o'chirish (agar mavjud bo'lsa)
DROP TABLE IF EXISTS medicines CASCADE;
DROP TABLE IF EXISTS settings CASCADE;

-- 2. Settings jadvali - har bir dorixonaga alohida sozlamalar
CREATE TABLE settings (
    id SERIAL PRIMARY KEY,
    pharmacy_id INT NOT NULL DEFAULT 1,
    key VARCHAR(50) NOT NULL,
    value TEXT,
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(key, pharmacy_id)  -- Har bir dorixonada bitta key
);

-- 3. Medicines jadvali - har bir dori dorixona ID bilan
CREATE TABLE medicines (
    id SERIAL PRIMARY KEY,
    pharmacy_id INT NOT NULL DEFAULT 1,
    name VARCHAR(255) NOT NULL,
    price INT NOT NULL DEFAULT 0,
    count INT NOT NULL DEFAULT 0,
    manufacturer VARCHAR(255),
    phone VARCHAR(50),
    address TEXT,
    description TEXT,
    category VARCHAR(100),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(name, pharmacy_id)  -- Har bir dorixonada bitta nom
);

-- 4. Index'lar - tezroq qidirish uchun
CREATE INDEX idx_medicines_name ON medicines(name);
CREATE INDEX idx_medicines_pharmacy ON medicines(pharmacy_id);
CREATE INDEX idx_medicines_category ON medicines(category);
CREATE INDEX idx_settings_pharmacy ON settings(pharmacy_id);

-- 5. Dastlabki ma'lumotlar - 3 ta dorixona nomlari
INSERT INTO settings (pharmacy_id, key, value) VALUES
    (1, 'name', 'Dorixona 1'),
    (2, 'name', 'Dorixona 2'),
    (3, 'name', 'Dorixona 3');

-- 6. Izohlar - jadvallar haqida ma'lumot
COMMENT ON TABLE medicines IS '3 ta dorixonaning dori bazasi';
COMMENT ON TABLE settings IS '3 ta dorixonaning sozlamalari';
COMMENT ON COLUMN medicines.pharmacy_id IS 'Dorixona raqami (1, 2, 3)';
COMMENT ON COLUMN settings.pharmacy_id IS 'Dorixona raqami (1, 2, 3)';

-- 7. Tasdiqlash xabari
DO $$
BEGIN
    RAISE NOTICE 'Migration muvaffaqiyatli bajarildi';
    RAISE NOTICE '✅ settings jadvali yaratildi';
    RAISE NOTICE '✅ medicines jadvali yaratildi';
    RAISE NOTICE '✅ 4 ta index yaratildi';
    RAISE NOTICE '✅ 3 ta dorixona nomi qo''shildi';
END $$;