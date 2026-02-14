-- Migration Rollback: Barcha jadvallarni o'chirish
-- Bu fayl migration'ni bekor qilish uchun ishlatiladi

-- 1. Indexlarni o'chirish (agar mavjud bo'lsa)
DROP INDEX IF EXISTS idx_medicines_name;
DROP INDEX IF EXISTS idx_medicines_pharmacy;
DROP INDEX IF EXISTS idx_medicines_category;
DROP INDEX IF EXISTS idx_settings_pharmacy;

-- 2. Jadvallarni o'chirish (CASCADE - bog'liq ma'lumotlarni ham o'chiradi)
DROP TABLE IF EXISTS medicines CASCADE;
DROP TABLE IF EXISTS settings CASCADE;

-- 3. Tasdiqlash xabari (psql'da ko'rinadi)
DO $$
BEGIN
    RAISE NOTICE 'Migration rollback muvaffaqiyatli bajarildi';
    RAISE NOTICE 'Barcha jadvallar va indexlar o''chirildi';
END $$;