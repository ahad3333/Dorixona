package excel

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

func parseNumber(s string) int {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", ".")
	
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int(f)
}

// detectCategory - dori nomidan kategoriyani aniqlash
func detectCategory(medicineName string) string {
	name := strings.ToLower(medicineName)
	
	categories := map[string][]string{
		"Antibiotik": {"азитромицин", "амоксициллин", "цефтриаксон", "ципрофлоксацин", "левофлоксацин", "доксициклин", "эритромицин"},
		"Og'riq qoldiruvchi": {"анальгин", "парацетамол", "ибупрофен", "кетопрофен", "диклофенак", "нимесулид", "кеторол"},
		"Kardio": {"амлодипин", "эналаприл", "лозартан", "валсартан", "бисопролол", "метопролол", "амлесса", "аторвастатин"},
		"Diabet": {"метформин", "глибенкламид", "гликлазид", "инсулин", "глимепирид"},
		"Shamolash": {"аскорил", "амброксол", "бромгексин", "мукалтин", "гербион", "синекод", "лазолван"},
		"Qorin": {"омепразол", "ранитидин", "мезим", "панкреатин", "эспумизан", "смекта", "линекс", "фестал"},
		"Allergiya": {"супрастин", "лоратадин", "цетиризин", "тавегил", "зодак", "кларитин"},
		"Vitamin": {"аевит", "компливит", "мультитабс", "витрум", "центрум", "кальций", "магний"},
		"Nerv tizimi": {"фенозепам", "адаптол", "глицин", "афобазол", "ново-пассит", "персен"},
		"Antivirus": {"арбидол", "кагоцел", "циклоферон", "ингавирин", "анаферон"},
		"Dermatalogiya": {"акридерм", "тридерм", "клотримазол", "синтомицин", "левомеколь"},
		"Oftalmologiya": {"альбуцид", "тобрекс", "визин", "систейн", "тауфон"},
		"Ginekologiya": {"утрожестан", "дюфастон", "тержинан", "клотримазол", "пимафуцин"},
	}
	
	for category, keywords := range categories {
		for _, keyword := range keywords {
			if strings.Contains(name, keyword) {
				return category
			}
		}
	}
	
	return "Boshqa"
}

func detectDescription(medicineName, category string) string {
	descriptions := map[string]string{
		"Antibiotik":        "Bakterial infeksiyalarni davolash uchun",
		"Og'riq qoldiruvchi": "Og'riq va yallig'lanishni kamaytiradi",
		"Kardio":            "Yurak-qon tomir kasalliklarini davolash",
		"Diabet":            "Qandli diabet davolash uchun",
		"Shamolash":         "Yo'tal va bronxitni davolash",
		"Qorin":             "Oshqozon-ichak tizimi kasalliklari uchun",
		"Allergiya":         "Allergik reaktsiyalarni kamaytiradi",
		"Vitamin":           "Organizm uchun zarur vitaminlar",
		"Nerv tizimi":       "Asab tizimi va stressni davolash",
		"Antivirus":         "Virus infeksiyalarini davolash",
		"Dermatalogiya":     "Teri kasalliklarini davolash",
		"Oftalmologiya":     "Ko'z kasalliklarini davolash",
		"Ginekologiya":      "Ayollar salomatligi uchun",
	}
	
	if desc, ok := descriptions[category]; ok {
		return desc
	}
	return "Shifo-dori vositasi"
}

func parseRowData(rowStr string) (num int, name string, count, price int, manufacturer string, ok bool) {
	rowStr = strings.TrimSpace(rowStr)
	if rowStr == "" {
		return
	}
	
	pattern1 := regexp.MustCompile(`^(\d+)\s+(.+?)\s+\d+\s+[\d,]+\s+([\d,]+)\s+([\d\s,]+)\s+[\d\s,]+\s*(.*)$`)
	matches := pattern1.FindStringSubmatch(rowStr)
	
	if len(matches) >= 5 {
		num, _ = strconv.Atoi(matches[1])
		name = strings.TrimSpace(matches[2])
		count = parseNumber(matches[3])
		price = parseNumber(matches[4])
		manufacturer = strings.TrimSpace(matches[5])
		ok = true
		return
	}
	
	pattern2 := regexp.MustCompile(`^(\d+)\s+(.+?)\s+([\d,]+)\s+([\d\s,]+)\s+[\d\s,]+\s*(.*)$`)
	matches = pattern2.FindStringSubmatch(rowStr)
	
	if len(matches) >= 5 {
		num, _ = strconv.Atoi(matches[1])
		name = strings.TrimSpace(matches[2])
		count = parseNumber(matches[3])
		price = parseNumber(matches[4])
		manufacturer = strings.TrimSpace(matches[5])
		ok = true
		return
	}
	
	return
}

func sendTelegramMessage(botToken, chatID, message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    message,
		"parse_mode": "HTML",
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: %s", string(body))
	}
	
	return nil
}

// Medicine struct - dorilarni saqlash uchun
type Medicine struct {
	Name         string
	Price        int
	Count        int
	Manufacturer string
	Phone        string
	Address      string
	Description  string
	Category     string
	PharmacyID   int
}

func UploadExcel(db *sql.DB, fileName string, fileData io.Reader, botToken, chatID, phone, address string, pharmacyID int) error {
	tmp := "upload_tmp.xlsx"
	
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, fileData)
	out.Close()
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	f, err := excelize.OpenFile(tmp)
	if err != nil {
		return fmt.Errorf("fayl ochilmadi: %v", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return fmt.Errorf("sheet topilmadi")
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return fmt.Errorf("qatorlar o'qilmadi: %v", err)
	}

	fmt.Println("📊 Jami qatorlar:", len(rows))

	var rawRows []string
	for _, row := range rows {
		fullRow := strings.Join(row, " ")
		fullRow = strings.TrimSpace(fullRow)
		if fullRow != "" {
			rawRows = append(rawRows, fullRow)
		}
	}

	fmt.Println("\n🔍 Birinchi 10 qator:")
	for i := 0; i < 10 && i < len(rawRows); i++ {
		fmt.Printf("Qator %d: %s\n", i+1, rawRows[i])
	}

	// Parse qilingan dorilarni yig'ish (map orqali dublikatlarni oldini olish)
	medicinesMap := make(map[string]Medicine) // key = dori nomi
	skipped := 0
	duplicates := 0
	failedRows := []string{}
	categoryStats := make(map[string]int)

	for i, rowStr := range rawRows {
		num, name, count, price, mfr, ok := parseRowData(rowStr)
		
		if !ok || num == 0 {
			skipped++
			if len(failedRows) < 10 {
				failedRows = append(failedRows, fmt.Sprintf("Qator %d: %s", i+1, rowStr))
			}
			continue
		}
		
		if mfr == "" {
			mfr = "Unknown"
		}

		category := detectCategory(name)
		description := detectDescription(name, category)
		categoryStats[category]++

		// Agar bu dori allaqachon mavjud bo'lsa, oxirgi qiymatni saqlash
		if _, exists := medicinesMap[name]; exists {
			duplicates++
		}

		medicinesMap[name] = Medicine{
			Name:         name,
			Price:        price,
			Count:        count,
			Manufacturer: mfr,
			Phone:        phone,
			Address:      address,
			Description:  description,
			Category:     category,
			PharmacyID:   pharmacyID,
		}
	}

	// Map'dan slice ga o'tkazish
	var medicines []Medicine
	for _, med := range medicinesMap {
		medicines = append(medicines, med)
	}

	fmt.Printf("\n✅ Parse qilindi: %d ta dori\n", len(medicines))
	if duplicates > 0 {
		fmt.Printf("🔄 Dublikatlar o'chirildi: %d ta (oxirgi qiymat saqlandi)\n", duplicates)
	}
	fmt.Printf("⏭️  O'tkazildi: %d ta\n", skipped)

	// BATCH INSERT - barcha dorilarni bir vaqtda yuklash
	if len(medicines) > 0 {
		saved, err := batchInsertMedicines(db, medicines)
		if err != nil {
			return fmt.Errorf("batch insert xato: %v", err)
		}

		fmt.Printf("\n📈 NATIJA:\n")
		fmt.Printf("✅ Saqlandi: %d ta\n", saved)
		
		fmt.Println("\n📊 Kategoriyalar bo'yicha:")
		for cat, count := range categoryStats {
			fmt.Printf("  %s: %d ta\n", cat, count)
		}
		
		if len(failedRows) > 0 {
			fmt.Println("\n🔍 Parse qilinmagan qatorlar:")
			for _, row := range failedRows {
				fmt.Println(row)
			}
		}

		// Telegram xabar
		if botToken != "" && chatID != "" {
			message := fmt.Sprintf(
				"📊 <b>Excel fayl yuklandi</b>\n\n"+
				"✅ Saqlandi: <b>%d</b> ta dori\n"+
				"📁 Fayl: <code>%s</code>",
				saved, fileName,
			)
			
			if duplicates > 0 {
				message += fmt.Sprintf("\n🔄 Dublikatlar: <b>%d</b> ta (oxirgi qiymat saqlandi)", duplicates)
			}
			
			err := sendTelegramMessage(botToken, chatID, message)
			if err != nil {
				fmt.Printf("⚠️ Telegram xabar yuborilmadi: %v\n", err)
			} else {
				fmt.Println("✅ Telegram botga xabar yuborildi")
			}
		}
	}

	return nil
}

// batchInsertMedicines - barcha dorilarni bir query bilan saqlash
func batchInsertMedicines(db *sql.DB, medicines []Medicine) (int, error) {
	if len(medicines) == 0 {
		return 0, nil
	}

	fmt.Println("\n🚀 Batch insert boshlandi...")
	
	// Transaction boshlash
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("transaction boshlanmadi: %v", err)
	}
	defer tx.Rollback()

	// Batch size - 100 tadan yuklash
	batchSize := 100
	saved := 0

	for i := 0; i < len(medicines); i += batchSize {
		end := i + batchSize
		if end > len(medicines) {
			end = len(medicines)
		}

		batch := medicines[i:end]
		
		// VALUES qismini yaratish
		var valueStrings []string
		var valueArgs []interface{}
		argPos := 1

		for _, med := range batch {
			valueStrings = append(valueStrings, fmt.Sprintf(
				"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, NOW())",
				argPos, argPos+1, argPos+2, argPos+3, argPos+4, argPos+5, argPos+6, argPos+7, argPos+8,
			))
			valueArgs = append(valueArgs, 
				med.Name, med.Price, med.Count, med.Manufacturer, 
				med.Phone, med.Address, med.Description, med.Category, med.PharmacyID,
			)
			argPos += 9
		}

		// Bitta katta INSERT query
		query := `
			INSERT INTO medicines (name, price, count, manufacturer, phone, address, description, category, pharmacy_id, updated_at)
			VALUES ` + strings.Join(valueStrings, ", ") + `
			ON CONFLICT (name, pharmacy_id) DO UPDATE SET
				price = EXCLUDED.price,
				count = EXCLUDED.count,
				manufacturer = EXCLUDED.manufacturer,
				phone = EXCLUDED.phone,
				address = EXCLUDED.address,
				description = EXCLUDED.description,
				category = EXCLUDED.category,
				updated_at = NOW()
		`

		_, err := tx.Exec(query, valueArgs...)
		if err != nil {
			return saved, fmt.Errorf("batch insert xato: %v", err)
		}

		saved += len(batch)
		fmt.Printf("  ✅ %d/%d yuklandi\n", saved, len(medicines))
	}

	// Transaction ni commit qilish
	if err := tx.Commit(); err != nil {
		return saved, fmt.Errorf("commit xato: %v", err)
	}

	fmt.Println("✅ Batch insert tugadi!")
	return saved, nil
}