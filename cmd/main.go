package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"testuchun/internal/excel"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var (
	superAdminID int64   // ADMIN_ID_1 - barcha dorixonalarga ruxsat
	adminIDs     []int64 // ADMIN_ID_2, ADMIN_ID_3, ADMIN_ID_4
)

// Har bir adminga tegishli dorixona ID
var adminPharmacy = make(map[int64]int) // userID -> pharmacy_id

// Super admin uchun upload session (keyingi Excel faylni qaysi dorixonaga yuklash)
var uploadSession = make(map[int64]int) // userID -> pharmacy_id

// translitToRussian - lotin harflarni kirillga o'giradi
func translitToRussian(text string) string {
	replacements := map[string]string{
		"a": "а", "b": "б", "v": "в", "g": "г", "d": "д", "e": "е", "yo": "ё",
		"zh": "ж", "z": "з", "i": "и", "y": "й", "k": "к", "l": "л", "m": "м",
		"n": "н", "o": "о", "p": "п", "r": "р", "s": "с", "t": "т", "u": "у",
		"f": "ф", "h": "х", "ts": "ц", "ch": "ч", "sh": "ш", "shch": "щ",
		"yu": "ю", "ya": "я",
		
		"A": "А", "B": "Б", "V": "В", "G": "Г", "D": "Д", "E": "Е", "Yo": "Ё",
		"Zh": "Ж", "Z": "З", "I": "И", "Y": "Й", "K": "К", "L": "Л", "M": "М",
		"N": "Н", "O": "О", "P": "П", "R": "Р", "S": "С", "T": "Т", "U": "У",
		"F": "Ф", "H": "Х", "Ts": "Ц", "Ch": "Ч", "Sh": "Ш", "Shch": "Щ",
		"Yu": "Ю", "Ya": "Я",
		
		"x": "кс", "w": "в", "q": "к", "c": "к",
		"X": "Кс", "W": "В", "Q": "К", "C": "К",
	}
	
	result := text
	for lat, cyr := range replacements {
		if len(lat) > 1 {
			result = strings.ReplaceAll(result, lat, cyr)
		}
	}
	for lat, cyr := range replacements {
		if len(lat) == 1 {
			result = strings.ReplaceAll(result, lat, cyr)
		}
	}
	
	return result
}

// isSuperAdmin - super admin ekanligini tekshirish
func isSuperAdmin(userID int64) bool {
	return userID == superAdminID
}

// isAdmin - admin ekanligini tekshirish (super yoki oddiy)
func isAdmin(userID int64) bool {
	if isSuperAdmin(userID) {
		return true
	}
	for _, adminID := range adminIDs {
		if adminID == userID {
			return true
		}
	}
	return false
}

// getPharmacyID - admin qaysi dorixonaga tegishli ekanligini aniqlash
func getPharmacyID(userID int64) int {
	if isSuperAdmin(userID) {
		return 0 // super admin barcha dorixonalarga kiradi
	}
	if pid, ok := adminPharmacy[userID]; ok {
		return pid
	}
	return 0
}

// getSetting - settings dan qiymat olish (dorixona bo'yicha)
func getSetting(db *sql.DB, key string, pharmacyID int) string {
	var value string
	err := db.QueryRow("SELECT value FROM settings WHERE key = $1 AND pharmacy_id = $2", key, pharmacyID).Scan(&value)
	if err != nil {
		return ""
	}
	return value
}

// updateSetting - settings ni yangilash (dorixona bo'yicha)
func updateSetting(db *sql.DB, key, value string, pharmacyID int) error {
	_, err := db.Exec(`
		INSERT INTO settings (key, value, pharmacy_id, updated_at) 
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (key, pharmacy_id) DO UPDATE SET value = $2, updated_at = NOW()
	`, key, value, pharmacyID)
	return err
}

// getAllPharmacies - barcha dorixonalar ro'yxati
func getAllPharmacies(db *sql.DB) map[int]string {
	pharmacies := make(map[int]string)
	
	rows, err := db.Query(`
		SELECT DISTINCT pharmacy_id, value 
		FROM settings 
		WHERE key = 'name' 
		ORDER BY pharmacy_id
	`)
	if err != nil {
		return pharmacies
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err == nil {
			pharmacies[id] = name
		}
	}

	return pharmacies
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(".env topilmadi")
	}

	botToken := os.Getenv("BOT_TOKEN")
	
	// Super admin va oddiy adminlarni o'qish
	superAdminID, _ = strconv.ParseInt(os.Getenv("ADMIN_ID_1"), 10, 64)
	admin2, _ := strconv.ParseInt(os.Getenv("ADMIN_ID_2"), 10, 64)
	admin3, _ := strconv.ParseInt(os.Getenv("ADMIN_ID_3"), 10, 64)
	admin4, _ := strconv.ParseInt(os.Getenv("ADMIN_ID_4"), 10, 64)
	
	adminIDs = []int64{admin2, admin3, admin4}
	
	// Har bir adminga dorixona biriktirish
	adminPharmacy[admin2] = 1 // Dorixona 1
	adminPharmacy[admin3] = 2 // Dorixona 2
	adminPharmacy[admin4] = 3 // Dorixona 3

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	db := connectDB()
	defer db.Close()

	// Healthcheck endpoint (Railway uchun)
	go func() {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})
		
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Pharmacy Bot is running"))
		})
		
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		
		fmt.Printf("🌐 Health check server running on :%s\n", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Printf("⚠️ Health check server error: %v\n", err)
		}
	}()

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-c
		fmt.Println("\n🛑 Bot to'xtatilmoqda...")
		bot.StopReceivingUpdates()
		db.Close()
		fmt.Println("✅ Bot to'xtatildi")
		os.Exit(0)
	}()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	fmt.Println("✅ Bot ishga tushdi...")
	fmt.Printf("🤖 Bot username: @%s\n", bot.Self.UserName)
	fmt.Printf("👑 Super Admin: %d\n", superAdminID)
	fmt.Printf("👥 Dorixona Adminlari:\n")
	for _, aid := range adminIDs {
		fmt.Printf("   - %d (Dorixona %d)\n", aid, adminPharmacy[aid])
	}

	for update := range updates {
		if update.Message == nil {
			continue
		}

		userID := update.Message.From.ID

		// /start komandasi
		if update.Message.Text == "/start" {
			// Barcha dorixonalarning ma'lumotlarini ko'rsatish
			var welcomeMsg strings.Builder
			welcomeMsg.WriteString("🏥 <b>Dorixonalar tarmog'iga xush kelibsiz!</b>\n\n")
			welcomeMsg.WriteString("💊 <b>Dori qidirish:</b>\n")
			welcomeMsg.WriteString("Qidirmoqchi bo'lgan doringiz nomini yozing\n\n")
			
			pharmacies := getAllPharmacies(db)
			if len(pharmacies) > 0 {
				welcomeMsg.WriteString("📍 <b>Bizning filiallar:</b>\n\n")
				for id := 1; id <= 3; id++ {
					name := getSetting(db, "name", id)
					phone := getSetting(db, "phone", id)
					address := getSetting(db, "address", id)
					
					if name == "" {
						name = fmt.Sprintf("Dorixona %d", id)
					}
					
					welcomeMsg.WriteString(fmt.Sprintf("🏪 <b>%s</b>\n", name))
					if phone != "" {
						welcomeMsg.WriteString(fmt.Sprintf("📞 %s\n", phone))
					}
					if address != "" {
						welcomeMsg.WriteString(fmt.Sprintf("📍 %s\n", address))
					}
					welcomeMsg.WriteString("\n")
				}
			}
			
			welcomeMsg.WriteString("✍️ Dori nomini yozing va qidiruvni boshlang!")
			
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, welcomeMsg.String())
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// /help komandasi
		if update.Message.Text == "/help" {
			helpMsg := 
				"📖 <b>Yordam</b>\n\n" +
				"🔍 <b>Qidirish:</b>\n" +
				"Dori nomini rus yoki lotin harflarida yozing\n\n" +
				"💡 <b>Maslahatlar:</b>\n" +
				"• To'liq nom yozmasangiz ham bo'ladi\n" +
				"• Katta-kichik harf farqi yo'q\n" +
				"• Bir necha so'z bilan qidiring\n\n" +
				"📋 Qidiruvda barcha filiallardan natija chiqadi"
			
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, helpMsg)
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// /ahad - MA'LUMOTLARNI O'CHIRISH
		if update.Message.Text == "/ahad" {
			if !isAdmin(userID) {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Siz admin emassiz"))
				continue
			}

			var confirmMsg string
			
			if isSuperAdmin(userID) {
				// Super admin - dorixona tanlash kerak
				confirmMsg = 
					"⚠️ <b>DIQQAT!</b>\n\n" +
					"Qaysi dorixonaning ma'lumotlarini o'chirmoqchisiz?\n\n" +
					"<code>/ahad 1</code> - Dorixona 1 ning barcha dorilarini o'chirish\n" +
					"<code>/ahad 2</code> - Dorixona 2 ning barcha dorilarini o'chirish\n" +
					"<code>/ahad 3</code> - Dorixona 3 ning barcha dorilarini o'chirish\n" +
					"<code>/ahad all</code> - BARCHA dorixonalarning ma'lumotlarini o'chirish\n\n" +
					"⚠️ Bu amaliyot QAYTARIB BO'LMAYDI!"
			} else {
				// Oddiy admin - faqat o'z dorixonasi
				pharmacyID := getPharmacyID(userID)
				pharmacyName := getSetting(db, "name", pharmacyID)
				if pharmacyName == "" {
					pharmacyName = fmt.Sprintf("Dorixona %d", pharmacyID)
				}
				
				confirmMsg = fmt.Sprintf(
					"⚠️ <b>DIQQAT!</b>\n\n"+
					"Siz <b>%s</b> ning barcha dorilarini o'chirmoqchisiz\n\n"+
					"🗑 Barcha dorilar o'chiriladi\n\n"+
					"Bu amaliyotni <b>QAYTARIB BO'LMAYDI!</b>\n\n"+
					"Davom etish uchun:\n"+
					"<code>/ahad_confirm</code>\n\n"+
					"Bekor qilish uchun:\n"+
					"<code>/cancel</code>",
					pharmacyName,
				)
			}
			
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, confirmMsg)
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// /ahad 1, /ahad 2, /ahad 3, /ahad all - Super admin uchun dorixona tanlash
		if strings.HasPrefix(update.Message.Text, "/ahad ") {
			if !isSuperAdmin(userID) {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Bu komanda faqat super admin uchun"))
				continue
			}

			parts := strings.Split(strings.TrimSpace(update.Message.Text), " ")
			if len(parts) != 2 {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Noto'g'ri format"))
				continue
			}

			target := parts[1]
			var confirmMsg string

			if target == "all" {
				// Barcha dorixonalarni o'chirish
				uploadSession[userID] = -1 // -1 = barcha dorixonalar
				
				confirmMsg = 
					"⚠️ <b>JUDA KATTA DIQQAT!</b>\n\n" +
					"Siz <b>BARCHA 3 TA DORIXONANING</b> ma'lumotlarini o'chirmoqchisiz:\n\n" +
					"🗑 Dorixona 1 - barcha dorilar\n" +
					"🗑 Dorixona 2 - barcha dorilar\n" +
					"🗑 Dorixona 3 - barcha dorilar\n\n" +
					"Bu amaliyotni <b>QAYTARIB BO'LMAYDI!</b>\n\n" +
					"Davom etish uchun:\n" +
					"<code>/ahad_confirm</code>\n\n" +
					"Bekor qilish uchun:\n" +
					"<code>/cancel</code>"
			} else {
				// Bitta dorixonani o'chirish
				pharmacyID, err := strconv.Atoi(target)
				if err != nil || pharmacyID < 1 || pharmacyID > 3 {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Dorixona raqami 1, 2, 3 yoki 'all' bo'lishi kerak"))
					continue
				}

				uploadSession[userID] = pharmacyID // Session'da saqlab qo'yish
				
				pharmacyName := getSetting(db, "name", pharmacyID)
				if pharmacyName == "" {
					pharmacyName = fmt.Sprintf("Dorixona %d", pharmacyID)
				}

				confirmMsg = fmt.Sprintf(
					"⚠️ <b>DIQQAT!</b>\n\n"+
					"Siz <b>%s</b> ning barcha dorilarini o'chirmoqchisiz\n\n"+
					"🗑 Barcha dorilar o'chiriladi\n\n"+
					"Bu amaliyotni <b>QAYTARIB BO'LMAYDI!</b>\n\n"+
					"Davom etish uchun:\n"+
					"<code>/ahad_confirm</code>\n\n"+
					"Bekor qilish uchun:\n"+
					"<code>/cancel</code>",
					pharmacyName,
				)
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, confirmMsg)
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// /ahad_confirm - Tasdiqlangan o'chirish
		if update.Message.Text == "/ahad_confirm" {
			if !isAdmin(userID) {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Siz admin emassiz"))
				continue
			}

			var pharmacyID int
			var deleteAll bool

			if isSuperAdmin(userID) {
				// Super admin uchun session'dan olish
				if pid, ok := uploadSession[userID]; ok {
					if pid == -1 {
						deleteAll = true
					} else {
						pharmacyID = pid
					}
					delete(uploadSession, userID) // Session tozalash
				} else {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, 
						"❌ Avval /ahad 1, /ahad 2, /ahad 3 yoki /ahad all ni tanlang"))
					continue
				}
			} else {
				// Oddiy admin - faqat o'z dorixonasi
				pharmacyID = getPharmacyID(userID)
			}

			var medicinesCount int64
			var successMsg string

			if deleteAll {
				// Barcha dorixonalarning dorilarini o'chirish
				db.QueryRow("SELECT COUNT(*) FROM medicines").Scan(&medicinesCount)
				
				_, err := db.Exec("TRUNCATE TABLE medicines RESTART IDENTITY")
				if err != nil {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ O'chirishda xato: "+err.Error()))
					continue
				}

				successMsg = fmt.Sprintf(
					"✅ <b>Barcha dorixonalar tozalandi!</b>\n\n"+
					"🗑 O'chirilgan dorilar: <b>%d</b> ta\n\n"+
					"📝 Endi qaytadan Excel yuklashingiz mumkin",
					medicinesCount,
				)
			} else {
				// Bitta dorixonaning dorilarini o'chirish
				db.QueryRow("SELECT COUNT(*) FROM medicines WHERE pharmacy_id = $1", pharmacyID).Scan(&medicinesCount)
				
				_, err := db.Exec("DELETE FROM medicines WHERE pharmacy_id = $1", pharmacyID)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ O'chirishda xato: "+err.Error()))
					continue
				}

				pharmacyName := getSetting(db, "name", pharmacyID)
				if pharmacyName == "" {
					pharmacyName = fmt.Sprintf("Dorixona %d", pharmacyID)
				}

				successMsg = fmt.Sprintf(
					"✅ <b>%s tozalandi!</b>\n\n"+
					"🗑 O'chirilgan dorilar: <b>%d</b> ta\n\n"+
					"📝 Endi qaytadan Excel yuklashingiz mumkin",
					pharmacyName, medicinesCount,
				)
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, successMsg)
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// /cancel - Bekor qilish
		if update.Message.Text == "/cancel" {
			// Session tozalash
			delete(uploadSession, userID)
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "✅ Bekor qilindi"))
			continue
		}

		// /admin - admin panel
		if update.Message.Text == "/admin" {
			if !isAdmin(userID) {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Siz admin emassiz"))
				continue
			}

			var adminMsg strings.Builder
			adminMsg.WriteString("⚙️ <b>Admin Panel</b>\n\n")

			if isSuperAdmin(userID) {
				// Super admin - barcha dorixonalarni ko'radi
				adminMsg.WriteString("👑 <b>Super Admin</b>\n\n")
				
				for id := 1; id <= 3; id++ {
					name := getSetting(db, "name", id)
					phone := getSetting(db, "phone", id)
					address := getSetting(db, "address", id)
					
					if name == "" {
						name = fmt.Sprintf("Dorixona %d", id)
					}
					
					adminMsg.WriteString(fmt.Sprintf("🏪 <b>%s</b>\n", name))
					adminMsg.WriteString(fmt.Sprintf("📞 %s\n", phone))
					adminMsg.WriteString(fmt.Sprintf("📍 %s\n\n", address))
				}
				
				adminMsg.WriteString("<b>O'zgartirish (dorixona raqami bilan):</b>\n")
				adminMsg.WriteString("🏪 Nom: <code>/setname 1 Markaziy dorixona</code>\n")
				adminMsg.WriteString("📞 Telefon: <code>/setphone 1 +998901234567</code>\n")
				adminMsg.WriteString("📍 Manzil: <code>/setaddress 1 https://maps...</code>\n\n")
				adminMsg.WriteString("<b>Excel yuklash:</b>\n")
				adminMsg.WriteString("1️⃣ <code>/upload 1</code> - Dorixona 1 tanlash\n")
				adminMsg.WriteString("2️⃣ Excel faylni yuborish\n")
				adminMsg.WriteString("\n<i>Har safar /upload qilishingiz kerak</i>")
			} else {
				// Oddiy admin - faqat o'z dorixonasini ko'radi
				pharmacyID := getPharmacyID(userID)
				name := getSetting(db, "name", pharmacyID)
				phone := getSetting(db, "phone", pharmacyID)
				address := getSetting(db, "address", pharmacyID)
				
				if name == "" {
					name = fmt.Sprintf("Dorixona %d", pharmacyID)
				}
				
				adminMsg.WriteString(fmt.Sprintf("🏪 <b>%s</b>\n\n", name))
				adminMsg.WriteString(fmt.Sprintf("📞 <b>Telefon:</b> %s\n", phone))
				adminMsg.WriteString(fmt.Sprintf("📍 <b>Manzil:</b> %s\n\n", address))
				
				adminMsg.WriteString("<b>O'zgartirish:</b>\n")
				adminMsg.WriteString("🏪 Nom: <code>/setname "+name+"</code>\n")
				adminMsg.WriteString("📞 Telefon: <code>/setphone +998901234567</code>\n")
				adminMsg.WriteString("📍 Manzil: <code>/setaddress https://maps...</code>\n\n")
				adminMsg.WriteString("📊 Excel yuklash: Faylni yuboring")
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, adminMsg.String())
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// /setname - dorixona nomini o'zgartirish
		if strings.HasPrefix(update.Message.Text, "/setname ") {
			if !isAdmin(userID) {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Siz admin emassiz"))
				continue
			}

			input := strings.TrimPrefix(update.Message.Text, "/setname ")
			input = strings.TrimSpace(input)
			
			var pharmacyID int
			var newName string
			
			if isSuperAdmin(userID) {
				// Super admin: /setname 1 Markaziy dorixona
				parts := strings.SplitN(input, " ", 2)
				if len(parts) < 2 {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, 
						"❌ Noto'g'ri format\n\n"+
						"To'g'ri format:\n"+
						"<code>/setname 1 Markaziy Dorixona</code>\n"+
						"<code>/setname 2 Chilonzor Filiali</code>"))
					continue
				}
				
				var err error
				pharmacyID, err = strconv.Atoi(parts[0])
				if err != nil || pharmacyID < 1 || pharmacyID > 3 {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Dorixona raqami 1, 2 yoki 3 bo'lishi kerak"))
					continue
				}
				newName = strings.TrimSpace(parts[1])
			} else {
				// Oddiy admin: /setname Markaziy dorixona
				pharmacyID = getPharmacyID(userID)
				newName = input
				
				if newName == "" {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, 
						"❌ Noto'g'ri format\n\n"+
						"To'g'ri format:\n"+
						"<code>/setname Markaziy Dorixona</code>"))
					continue
				}
			}

			err := updateSetting(db, "name", newName, pharmacyID)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Xatolik: "+err.Error()))
				continue
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, 
				fmt.Sprintf("✅ Dorixona %d nomi yangilandi!\n\n🏪 Yangi nom: <b>%s</b>", pharmacyID, newName))
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// /setphone - telefon o'zgartirish
		if strings.HasPrefix(update.Message.Text, "/setphone ") {
			if !isAdmin(userID) {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Siz admin emassiz"))
				continue
			}

			input := strings.TrimPrefix(update.Message.Text, "/setphone ")
			input = strings.TrimSpace(input)
			
			var pharmacyID int
			var newPhone string
			
			if isSuperAdmin(userID) {
				// Super admin: /setphone 1 +998901234567
				parts := strings.SplitN(input, " ", 2)
				if len(parts) < 2 {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, 
						"❌ Noto'g'ri format\n\n"+
						"To'g'ri format:\n"+
						"<code>/setphone 1 +998901234567</code>"))
					continue
				}
				
				var err error
				pharmacyID, err = strconv.Atoi(parts[0])
				if err != nil || pharmacyID < 1 || pharmacyID > 3 {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Dorixona raqami 1, 2 yoki 3 bo'lishi kerak"))
					continue
				}
				newPhone = strings.TrimSpace(parts[1])
			} else {
				// Oddiy admin: /setphone +998901234567
				pharmacyID = getPharmacyID(userID)
				newPhone = input
				
				if newPhone == "" {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, 
						"❌ Noto'g'ri format\n\n"+
						"To'g'ri format:\n"+
						"<code>/setphone +998901234567</code>"))
					continue
				}
			}

			err := updateSetting(db, "phone", newPhone, pharmacyID)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Xatolik: "+err.Error()))
				continue
			}

			// Barcha dorilarni yangilash
			_, err = db.Exec("UPDATE medicines SET phone = $1 WHERE pharmacy_id = $2", newPhone, pharmacyID)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "⚠️ Settings yangilandi, lekin dorilar yangilanmadi"))
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, 
				fmt.Sprintf("✅ Dorixona %d telefoni yangilandi!\n\n📞 Yangi: <code>%s</code>", pharmacyID, newPhone))
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// /setaddress - manzil o'zgartirish
		if strings.HasPrefix(update.Message.Text, "/setaddress ") {
			if !isAdmin(userID) {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Siz admin emassiz"))
				continue
			}

			input := strings.TrimPrefix(update.Message.Text, "/setaddress ")
			input = strings.TrimSpace(input)
			
			var pharmacyID int
			var newAddress string
			
			if isSuperAdmin(userID) {
				// Super admin: /setaddress 1 Toshkent, Amir Temur
				parts := strings.SplitN(input, " ", 2)
				if len(parts) < 2 {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, 
						"❌ Noto'g'ri format\n\n"+
						"To'g'ri format:\n"+
						"<code>/setaddress 1 Toshkent, Amir Temur 123</code>\n"+
						"<code>/setaddress 1 https://maps.app.goo.gl/xxx</code>"))
					continue
				}
				
				var err error
				pharmacyID, err = strconv.Atoi(parts[0])
				if err != nil || pharmacyID < 1 || pharmacyID > 3 {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Dorixona raqami 1, 2 yoki 3 bo'lishi kerak"))
					continue
				}
				newAddress = strings.TrimSpace(parts[1])
			} else {
				// Oddiy admin: /setaddress Toshkent, Amir Temur
				pharmacyID = getPharmacyID(userID)
				newAddress = input
				
				if newAddress == "" {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, 
						"❌ Noto'g'ri format\n\n"+
						"To'g'ri format:\n"+
						"<code>/setaddress Toshkent, Amir Temur 123</code>\n"+
						"<code>/setaddress https://maps.app.goo.gl/xxx</code>"))
					continue
				}
			}

			err := updateSetting(db, "address", newAddress, pharmacyID)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Xatolik: "+err.Error()))
				continue
			}

			// Barcha dorilarni yangilash
			_, err = db.Exec("UPDATE medicines SET address = $1 WHERE pharmacy_id = $2", newAddress, pharmacyID)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "⚠️ Settings yangilandi, lekin dorilar yangilanmadi"))
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, 
				fmt.Sprintf("✅ Dorixona %d manzili yangilandi!\n\n📍 Yangi: %s", pharmacyID, newAddress))
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// /upload - Super admin uchun dorixona tanlash
		if strings.HasPrefix(update.Message.Text, "/upload ") {
			if !isSuperAdmin(userID) {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Bu komanda faqat super admin uchun"))
				continue
			}

			parts := strings.Split(strings.TrimSpace(update.Message.Text), " ")
			if len(parts) != 2 {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, 
					"❌ Noto'g'ri format\n\n"+
					"To'g'ri format:\n"+
					"<code>/upload 1</code> - Dorixona 1 uchun\n"+
					"<code>/upload 2</code> - Dorixona 2 uchun\n"+
					"<code>/upload 3</code> - Dorixona 3 uchun"))
				continue
			}

			pharmacyID, err := strconv.Atoi(parts[1])
			if err != nil || pharmacyID < 1 || pharmacyID > 3 {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Dorixona raqami 1, 2 yoki 3 bo'lishi kerak"))
				continue
			}

			// Session'da saqlab qo'yish
			uploadSession[userID] = pharmacyID

			pharmacyName := getSetting(db, "name", pharmacyID)
			if pharmacyName == "" {
				pharmacyName = fmt.Sprintf("Dorixona %d", pharmacyID)
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, 
				fmt.Sprintf("✅ Tayyor!\n\n"+
					"📤 Endi <b>%s</b> uchun Excel faylni yuboring\n\n"+
					"⚠️ Faqat keyingi yuboradigan Excel fayl bu dorixonaga yuklanadi", pharmacyName))
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// Excel fayl yuklash
		if update.Message.Document != nil {
			if !isAdmin(userID) {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Siz admin emassiz"))
				continue
			}
			
			var pharmacyID int
			
			// Super admin uchun session'dan olish
			if isSuperAdmin(userID) {
				if pid, ok := uploadSession[userID]; ok {
					pharmacyID = pid
					// Session'ni tozalash (bir marta ishlatiladi)
					delete(uploadSession, userID)
				} else {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, 
						"⚠️ Avval dorixona raqamini belgilang!\n\n"+
						"<code>/upload 1</code> - Dorixona 1\n"+
						"<code>/upload 2</code> - Dorixona 2\n"+
						"<code>/upload 3</code> - Dorixona 3\n\n"+
						"Keyin Excel faylni yuboring"))
					continue
				}
			} else {
				// Oddiy admin uchun o'z dorixonasi
				pharmacyID = getPharmacyID(userID)
			}
			
			phone := getSetting(db, "phone", pharmacyID)
			address := getSetting(db, "address", pharmacyID)
			
			if phone == "" || address == "" {
				warningMsg := "⚠️ <b>Diqqat!</b>\n\n"
				if phone == "" {
					warningMsg += "📞 Telefon raqam kiritilmagan\n"
				}
				if address == "" {
					warningMsg += "📍 Manzil kiritilmagan\n"
				}
				warningMsg += fmt.Sprintf("\nIltimos, avval Dorixona %d uchun ma'lumotlarni kiriting:\n", pharmacyID)
				
				if isSuperAdmin(userID) {
					warningMsg += fmt.Sprintf("<code>/setphone %d +998901234567</code>\n", pharmacyID)
					warningMsg += fmt.Sprintf("<code>/setaddress %d https://maps...</code>", pharmacyID)
				} else {
					warningMsg += "<code>/setphone +998901234567</code>\n"
					warningMsg += "<code>/setaddress https://maps...</code>"
				}
				
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, warningMsg)
				msg.ParseMode = "HTML"
				bot.Send(msg)
				continue
			}
		
			file, _ := bot.GetFile(tgbotapi.FileConfig{FileID: update.Message.Document.FileID})
			url := file.Link(bot.Token)
		
			resp, err := http.Get(url)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Fayl yuklanmadi"))
				continue
			}
			defer resp.Body.Close()

			chatID := strconv.FormatInt(update.Message.Chat.ID, 10)
			err = excel.UploadExcel(db, update.Message.Document.FileName, resp.Body, botToken, chatID, phone, address, pharmacyID)
			if err != nil {
				fmt.Println("error:", err)
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ DB ga yozishda xato: "+err.Error()))
				continue
			}
		
			continue
		}

		// Matn bilan qidirish - BARCHA dorixonalardan
		if update.Message.Text != "" {
			search := strings.TrimSpace(update.Message.Text)
			if search == "" {
				continue
			}
		
			searchRussian := translitToRussian(search)
			
			rows, err := db.Query(`
				SELECT m.name, m.price, m.count, m.manufacturer, m.phone, m.address, 
				       m.description, m.category, m.pharmacy_id, s.value as pharmacy_name
				FROM medicines m
				LEFT JOIN settings s ON s.pharmacy_id = m.pharmacy_id AND s.key = 'name'
				WHERE m.name ILIKE $1 OR m.name ILIKE $2
				ORDER BY m.pharmacy_id, m.name
				LIMIT 30
			`, "%"+search+"%", "%"+searchRussian+"%")
			if err != nil {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ DB error"))
				continue
			}
		
			var response strings.Builder
			found := false
			for rows.Next() {
				var name, manufacturer, phone, address string
				var description, category, pharmacyName sql.NullString
				var price, count, pharmacyID int
				
				rows.Scan(&name, &price, &count, &manufacturer, &phone, &address, 
					&description, &category, &pharmacyID, &pharmacyName)
		
				// Dorixona nomini ko'rsatish
				if pharmacyName.Valid && pharmacyName.String != "" {
					response.WriteString(fmt.Sprintf("🏪 <b>%s</b>\n", pharmacyName.String))
				} else {
					response.WriteString(fmt.Sprintf("🏪 <b>Dorixona %d</b>\n", pharmacyID))
				}
				
				response.WriteString(fmt.Sprintf(
					"💊 <b>%s</b>\n"+
					"🧮 Miqdor: <b>%d</b> dona\n"+
					"💰 Narx: <b>%d</b> so'm\n"+
					"📞 Telefon: <code>%s</code>\n"+
					"📍 Manzil: %s\n",
					name, count, price, phone, address,
				))
				
				if category.Valid && category.String != "" {
					response.WriteString(fmt.Sprintf("🏷 Kategoriya: %s\n", category.String))
				}
				
				if description.Valid && description.String != "" {
					response.WriteString(fmt.Sprintf("ℹ️ %s\n", description.String))
				}
				
				response.WriteString("\n")
				found = true
			}
			rows.Close()
		
			if !found {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Topilmadi"))
			} else {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, response.String())
				msg.ParseMode = "HTML"
				bot.Send(msg)
			}
		}
	}
}

func connectDB() *sql.DB {
	// Birinchi DATABASE_URL ni tekshirish (Railway, Fly.io uchun)
	databaseURL := os.Getenv("DATABASE_URL")
	
	if databaseURL != "" {
		// Railway/Fly.io - to'g'ridan-to'g'ri URL
		db, err := sql.Open("postgres", databaseURL)
		if err != nil {
			log.Fatal("❌ DB ochilmadi:", err)
		}

		// Connection pool sozlash (Railway uchun optimizatsiya)
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(5 * time.Minute)

		if err = db.Ping(); err != nil {
			log.Fatal("❌ DB ping error:", err)
		}

		fmt.Println("✅ Database connected (Railway/Fly.io)")
		return db
	}
	
	// Local development - alohida parametrlar
	dsn := "host=" + os.Getenv("DB_HOST") +
		" port=" + os.Getenv("DB_PORT") +
		" user=" + os.Getenv("DB_USER") +
		" password=" + os.Getenv("DB_PASSWORD") +
		" dbname=" + os.Getenv("DB_NAME") +
		" sslmode=" + os.Getenv("DB_SSLMODE")

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("❌ DB ochilmadi:", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("❌ DB ping error:", err)
	}

	fmt.Println("✅ Database connected (Local)")
	return db
}